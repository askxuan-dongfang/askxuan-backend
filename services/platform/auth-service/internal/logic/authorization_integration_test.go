package logic

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/askxuan/auth-service/internal/model"
	"github.com/askxuan/auth-service/internal/svc"
	"github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Explicit opt-in only. The exact, separate schema prevents this concurrency
// fixture from sharing the interactive HTTP acceptance users or a real auth DB.
func TestAuthorizationMySQLTransactions(t *testing.T) {
	dsn := os.Getenv("AUTHORIZATION_TEST_DSN")
	if dsn == "" {
		t.Skip("set AUTHORIZATION_TEST_DSN to disposable askxuan_admin_test_auth_tx")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil || cfg.DBName != "askxuan_admin_test_auth_tx" {
		t.Fatal("refusing database outside isolated authorization transaction schema")
	}
	sqlx.DisableLog()
	db := sqlx.NewMysql(dsn)
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	var database string
	if db.QueryRowCtx(ctx, &database, "SELECT DATABASE()") != nil || database != cfg.DBName {
		t.Fatal("isolated authorization database unavailable")
	}
	ddls := []string{
		`CREATE TABLE IF NOT EXISTS role(id BIGINT PRIMARY KEY,name VARCHAR(64) NOT NULL,code VARCHAR(64) NOT NULL UNIQUE,description VARCHAR(255) NOT NULL DEFAULT '',authz_scope VARCHAR(16) NOT NULL DEFAULT 'platform',authz_version BIGINT NOT NULL DEFAULT 0,create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS permission(id BIGINT PRIMARY KEY,code VARCHAR(64) NOT NULL UNIQUE,name VARCHAR(64) NOT NULL DEFAULT '',resource VARCHAR(64) NOT NULL DEFAULT '',action VARCHAR(32) NOT NULL DEFAULT '') ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS role_permission(id BIGINT AUTO_INCREMENT PRIMARY KEY,role_id BIGINT NOT NULL,permission_id BIGINT NOT NULL,UNIQUE KEY uk_role_perm(role_id,permission_id)) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS admin_account(id BIGINT PRIMARY KEY,role_id BIGINT NOT NULL,status VARCHAR(16) NOT NULL,authz_version BIGINT NOT NULL DEFAULT 0) ENGINE=InnoDB`,
	}
	for _, ddl := range ddls {
		if _, err = db.ExecCtx(ctx, ddl); err != nil {
			t.Fatal("cannot create isolated transaction fixtures")
		}
	}
	for _, table := range []string{"role", "permission", "role_permission", "admin_account"} {
		var count int64
		if db.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM "+table) != nil || count != 0 {
			t.Fatal("isolated transaction schema must be empty; refusing to overwrite existing data")
		}
	}
	t.Cleanup(func() {
		for _, query := range []string{"DELETE FROM role_permission WHERE role_id IN (1,2)", "DELETE FROM admin_account WHERE id IN (1001,1002)", "DELETE FROM permission WHERE id IN (101,102)", "DELETE FROM role WHERE id IN (1,2)"} {
			if _, err := db.ExecCtx(context.Background(), query); err != nil {
				t.Error("could not remove isolated authorization fixture rows")
			}
		}
	})
	for _, query := range []string{
		`INSERT INTO role(id,name,code) VALUES(1,'Fixture super','platform_super'),(2,'Fixture custom','tx_reader')`,
		`INSERT INTO permission(id,code) VALUES(101,'platform.operations.read'),(102,'platform.products.read')`,
		`INSERT INTO role_permission(role_id,permission_id) VALUES(2,101)`,
		`INSERT INTO admin_account(id,role_id,status) VALUES(1001,1,'enabled'),(1002,1,'enabled')`,
	} {
		if _, err := db.ExecCtx(ctx, query); err != nil {
			t.Fatal("cannot seed isolated transaction fixture")
		}
	}
	s := &svc.ServiceContext{DB: db, RoleModel: model.NewRoleModel(db)}

	t.Run("same revision concurrent grants have one winner", func(t *testing.T) {
		gate := make(chan struct{})
		results := make(chan error, 2)
		var wg sync.WaitGroup
		for _, code := range []string{"platform.operations.read", "platform.products.read"} {
			wg.Add(1)
			go func(code string) {
				defer wg.Done()
				<-gate
				_, err := SetRolePermissions(ctx, s, 2, 0, []string{code})
				results <- err
			}(code)
		}
		close(gate)
		wg.Wait()
		close(results)
		success, conflict := 0, 0
		for err := range results {
			if err == nil {
				success++
			} else if errors.Is(err, ErrAuthorizationConflict) {
				conflict++
			} else {
				t.Fatalf("unexpected concurrent grant failure: %v", err)
			}
		}
		var revision, count int64
		if db.QueryRowCtx(ctx, &revision, "SELECT authz_version FROM role WHERE id=2") != nil || db.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM role_permission WHERE role_id=2") != nil || success != 1 || conflict != 1 || revision != 1 || count != 1 {
			t.Fatalf("grant transaction failed: success=%d conflict=%d revision=%d mappings=%d", success, conflict, revision, count)
		}
	})
	t.Run("missing catalog row rolls back version and old mappings", func(t *testing.T) {
		var before string
		if db.QueryRowCtx(ctx, &before, "SELECT p.code FROM role_permission rp JOIN permission p ON p.id=rp.permission_id WHERE rp.role_id=2") != nil {
			t.Fatal("missing original grant")
		}
		// users.read is a valid capability deliberately absent from this fixture DB.
		if _, err := SetRolePermissions(ctx, s, 2, 1, []string{"platform.users.read"}); err == nil {
			t.Fatal("missing permission row committed")
		}
		var after string
		var revision int64
		if db.QueryRowCtx(ctx, &after, "SELECT p.code FROM role_permission rp JOIN permission p ON p.id=rp.permission_id WHERE rp.role_id=2") != nil || db.QueryRowCtx(ctx, &revision, "SELECT authz_version FROM role WHERE id=2") != nil || after != before || revision != 1 {
			t.Fatal("failed grant did not roll back both mappings and revision")
		}
	})
	t.Run("two simultaneous super disables preserve one enabled account", func(t *testing.T) {
		gate := make(chan struct{})
		results := make(chan error, 2)
		var wg sync.WaitGroup
		for _, id := range []int64{1001, 1002} {
			wg.Add(1)
			go func(id int64) {
				defer wg.Done()
				<-gate
				results <- updateAccountStatus(ctx, s, &model.AdminAccount{Id: id, RoleId: 1}, model.AccountStatusDisabled)
			}(id)
		}
		close(gate)
		wg.Wait()
		close(results)
		success, protected := 0, 0
		for err := range results {
			if err == nil {
				success++
			} else if errors.Is(err, ErrLastSuper) {
				protected++
			} else {
				t.Fatalf("unexpected super transaction failure: %v", err)
			}
		}
		var enabled, revoked int64
		if db.QueryRowCtx(ctx, &enabled, "SELECT COUNT(*) FROM admin_account WHERE role_id=1 AND status='enabled'") != nil || db.QueryRowCtx(ctx, &revoked, "SELECT COUNT(*) FROM admin_account WHERE status='disabled' AND authz_version=1") != nil || success != 1 || protected != 1 || enabled != 1 || revoked != 1 {
			t.Fatalf("super invariant failed: success=%d protected=%d enabled=%d revoked=%d", success, protected, enabled, revoked)
		}
	})
	// Fixture schema names are checked both before the connection and by SELECT DATABASE().
	if !strings.HasPrefix(database, "askxuan_admin_test_") {
		t.Fatal("unreachable non-test schema")
	}
}
