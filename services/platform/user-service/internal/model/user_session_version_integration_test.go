package model

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// This test invokes the production status model on an opt-in disposable schema.
// It never truncates a table, seeds existing IDs, or connects outside loopback.
func TestUserBanRevisionMySQL(t *testing.T) {
	dsn := os.Getenv("ACCOUNT_SECURITY_TEST_DSN")
	if dsn == "" {
		dsn = os.Getenv("ACCOUNT_SYSTEM_TEST_DSN")
	}
	if dsn == "" {
		t.Skip("set ACCOUNT_SECURITY_TEST_DSN to isolated local account-system MySQL")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal("invalid fixture DSN")
	}
	isolated := os.Getenv("ACCOUNT_SYSTEM_TEST_ISOLATED") == "1" && cfg.DBName == "askxuan_account_test" && cfg.Net == "tcp" && strings.HasPrefix(cfg.Addr, "127.0.0.1:")
	legacy := cfg.Net == "tcp" && cfg.Addr == "127.0.0.1:63326" && cfg.DBName == "askxuan_auth"
	if !isolated && !legacy {
		t.Fatal("refusing non-fixture database")
	}
	cfg.DBName = "askxuan_user"
	db := sqlx.NewMysql(cfg.FormatDSN())
	sqlx.DisableLog()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var marker int64
	if err = db.QueryRowCtx(ctx, &marker, "SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name='askxuan_account_test'"); err != nil || marker != 1 {
		t.Fatal("disposable marker missing", err)
	}
	const id int64 = 920001
	var count int64
	if err = db.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM user WHERE id=?", id); err != nil || count != 0 {
		t.Fatal("fixture ID occupied; refusing overwrite", err)
	}
	if _, err = db.ExecCtx(ctx, "INSERT INTO user(id,nickname,password,status) VALUES(?,'security-fixture','unchanged-fixture-sentinel',1)", id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, err := db.ExecCtx(context.Background(), "DELETE FROM user WHERE id=? AND nickname='security-fixture'", id)
		if err != nil {
			t.Error(err)
		}
	})
	m := NewUserModel(db)
	assert := func(status int, revision int64) {
		t.Helper()
		var got struct {
			Status   int    `db:"status"`
			Revision int64  `db:"authz_version"`
			Password string `db:"password"`
		}
		if err := db.QueryRowCtx(ctx, &got, "SELECT status,authz_version,password FROM user WHERE id=?", id); err != nil || got.Status != status || got.Revision != revision || got.Password != "unchanged-fixture-sentinel" {
			t.Fatalf("unexpected state %+v err=%v", got, err)
		}
	}
	for _, state := range []struct {
		status  int
		version int64
	}{{0, 1}, {1, 2}} {
		var wg sync.WaitGroup
		errs := make(chan error, 24)
		start := make(chan struct{})
		for n := 0; n < 24; n++ {
			wg.Add(1)
			go func() { defer wg.Done(); <-start; errs <- m.UpdateStatus(ctx, id, state.status) }()
		}
		close(start)
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatal(err)
			}
		}
		assert(state.status, state.version)
	}
	if err = m.UpdateStatus(ctx, id, 9); err == nil {
		t.Fatal("unknown status accepted")
	}
	assert(1, 2)
}
