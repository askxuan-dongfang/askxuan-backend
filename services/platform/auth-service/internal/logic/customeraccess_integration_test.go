package logic

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/askxuan/auth-service/internal/model"
	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type delayedCustomerRead struct {
	model.UserReadonlyModel
	read, resume chan struct{}
	delayed      bool
}

func (m *delayedCustomerRead) FindByID(ctx context.Context, id int64) (*model.User, error) {
	row, err := m.UserReadonlyModel.FindByID(ctx, id)
	if !m.delayed {
		m.delayed = true
		close(m.read)
		select {
		case <-m.resume:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return row, err
}

func TestCustomerDurableAvailabilityMySQL(t *testing.T) {
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
	cfg.DBName = "askxuan_auth"
	db := sqlx.NewMysql(cfg.FormatDSN())
	sqlx.DisableLog()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var marker int64
	if err = db.QueryRowCtx(ctx, &marker, "SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name='askxuan_account_test'"); err != nil || marker != 1 {
		t.Fatal("disposable marker missing", err)
	}
	const id int64 = 920002
	var count int64
	if err = db.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM askxuan_user.user WHERE id=?", id); err != nil || count != 0 {
		t.Fatal("fixture ID occupied; refusing overwrite", err)
	}
	if _, err = db.ExecCtx(ctx, "INSERT INTO askxuan_user.user(id,nickname,status) VALUES(?,'durable-security-fixture',1)", id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, err := db.ExecCtx(context.Background(), "DELETE FROM askxuan_user.user WHERE id=? AND nickname='durable-security-fixture'", id)
		if err != nil {
			t.Error(err)
		}
	})
	sc := &svc.ServiceContext{DB: db, UserReadonlyModel: model.NewUserReadonlyModel(db)}
	sc.Config.Auth.AccessSecret = refreshTestSecret
	sc.Config.Auth.AccessExpire = 60
	sc.Config.Auth.RefreshExpire = 600
	login, err := NewLoginLogic(ctx, sc).IssueCustomer(id)
	if err != nil {
		t.Fatal(err)
	}
	access, err := common.ParseToken(refreshTestSecret, login.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = CurrentCustomerAccess(ctx, sc, access); err != nil {
		t.Fatal(err)
	}
	for _, status := range []int{0, 1} {
		if _, err = db.ExecCtx(ctx, "UPDATE askxuan_user.user SET status=?,authz_version=authz_version+1 WHERE id=?", status, id); err != nil {
			t.Fatal(err)
		}
		if _, err = CurrentCustomerAccess(ctx, sc, access); !errors.Is(err, common.ErrSessionRevoked) {
			t.Fatalf("old session survived status=%d: %v", status, err)
		}
	}
	fresh, err := NewLoginLogic(ctx, sc).IssueCustomer(id)
	if err != nil {
		t.Fatal(err)
	}
	freshAccess, _ := common.ParseToken(refreshTestSecret, fresh.AccessToken)
	if freshAccess.AccountVersion != 2 {
		t.Fatal("real DB revision was not signed")
	}
	if _, err = CurrentCustomerAccess(ctx, sc, freshAccess); err != nil {
		t.Fatal(err)
	}
	// Hold a real SQL snapshot from before a ban/unban, then finish sign-in after
	// the authority changed. The second read must refuse the delayed old login.
	delayed := &delayedCustomerRead{UserReadonlyModel: sc.UserReadonlyModel, read: make(chan struct{}), resume: make(chan struct{})}
	sc.UserReadonlyModel = delayed
	result := make(chan error, 1)
	go func() { _, err := NewLoginLogic(ctx, sc).IssueCustomer(id); result <- err }()
	select {
	case <-delayed.read:
	case <-ctx.Done():
		t.Fatal("login did not reach read gate")
	}
	if _, err = db.ExecCtx(ctx, "UPDATE askxuan_user.user SET authz_version=authz_version+2 WHERE id=?", id); err != nil {
		t.Fatal(err)
	}
	close(delayed.resume)
	if err = <-result; !errors.Is(err, common.ErrSessionRevoked) {
		t.Fatal("delayed pre-ban login returned success", err)
	}
}
