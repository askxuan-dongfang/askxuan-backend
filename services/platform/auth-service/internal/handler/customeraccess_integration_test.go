package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/askxuan/auth-service/internal/model"
	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/askxuan/common/identity"
	"github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestHumanIntrospectionUsesRealSIDAndDurableCustomerState(t *testing.T) {
	dsn, host := os.Getenv("ACCOUNT_SECURITY_TEST_DSN"), os.Getenv("ACCOUNT_SECURITY_TEST_REDIS")
	if dsn == "" {
		dsn = os.Getenv("ACCOUNT_SYSTEM_TEST_DSN")
	}
	if host == "" {
		host = os.Getenv("ACCOUNT_SYSTEM_TEST_REDIS")
	}
	if dsn == "" || host == "" {
		t.Skip("set isolated ACCOUNT_SECURITY_TEST_DSN and ACCOUNT_SECURITY_TEST_REDIS")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal("invalid fixture DSN")
	}
	isolated := os.Getenv("ACCOUNT_SYSTEM_TEST_ISOLATED") == "1" && cfg.DBName == "askxuan_account_test" && cfg.Net == "tcp" && strings.HasPrefix(cfg.Addr, "127.0.0.1:") && strings.HasPrefix(host, "127.0.0.1:")
	legacy := cfg.Net == "tcp" && cfg.Addr == "127.0.0.1:63326" && cfg.DBName == "askxuan_auth" && host == "127.0.0.1:63327"
	if !isolated && !legacy {
		t.Fatal("refusing non-fixture authorities")
	}
	cfg.DBName = "askxuan_auth"
	db := sqlx.NewMysql(cfg.FormatDSN())
	sqlx.DisableLog()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var marker int64
	if err = db.QueryRowCtx(ctx, &marker, "SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name='askxuan_account_test'"); err != nil || marker != 1 {
		t.Fatal("disposable marker missing", err)
	}
	const id int64 = 920003
	var count int64
	if err = db.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM askxuan_user.user WHERE id=?", id); err != nil || count != 0 {
		t.Fatal("fixture ID occupied; refusing overwrite", err)
	}
	if _, err = db.ExecCtx(ctx, "INSERT INTO askxuan_user.user(id,nickname,status) VALUES(?,'sid-security-fixture',1)", id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, err := db.ExecCtx(context.Background(), "DELETE FROM askxuan_user.user WHERE id=? AND nickname='sid-security-fixture'", id)
		if err != nil {
			t.Error(err)
		}
	})
	store := redis.MustNewRedis(redis.RedisConf{Host: host, Type: "node"})
	sid, err := identity.CreateSession(ctx, store, "user", id, 60)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := identity.RevokeSession(context.Background(), store, sid); err != nil {
			t.Error(err)
		}
	})
	sc := &svc.ServiceContext{DB: db, SessionRedis: store, UserReadonlyModel: model.NewUserReadonlyModel(db)}
	sc.Config.Auth.AccessSecret = "sid-state-handler-fixture-only-key"
	token, _ := common.GenAccessToken(sc.Config.Auth.AccessSecret, common.TokenInfo{UserId: id, UserType: "user", Roles: []string{"customer"}, ClientID: "customer", SessionID: sid}, 60)
	req := httptest.NewRequest("GET", "/api/v1/auth/access", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if _, _, err = currentHumanClaims(sc, req); err != nil {
		t.Fatal("valid real session rejected", err)
	}
	if _, _, err = currentAdminClaims(sc, req); !errors.Is(err, common.ErrRoleForbidden) {
		t.Fatal("consumer acquired administrator identity", err)
	}
	if _, err = db.ExecCtx(ctx, "UPDATE askxuan_user.user SET status=0,authz_version=authz_version+1 WHERE id=?", id); err != nil {
		t.Fatal(err)
	}
	if _, _, err = currentHumanClaims(sc, req); !errors.Is(err, common.ErrSessionRevoked) {
		t.Fatal("still-valid Redis SID bypassed user ban", err)
	}
	w := httptest.NewRecorder()
	imTokenHandler(sc)(w, req.Clone(ctx))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"code":40105`) {
		t.Fatal("direct IM-token issuance bypassed user ban", w.Body.String())
	}
	if _, err = db.ExecCtx(ctx, "UPDATE askxuan_user.user SET status=1,authz_version=authz_version+1 WHERE id=?", id); err != nil {
		t.Fatal(err)
	}
	if _, _, err = currentHumanClaims(sc, req); !errors.Is(err, common.ErrSessionRevoked) {
		t.Fatal("restoration revived old SID/version", err)
	}
	// A fresh version uses a live SID, then becomes invalid immediately at logout.
	fresh, _ := common.GenAccessToken(sc.Config.Auth.AccessSecret, common.TokenInfo{UserId: id, UserType: "user", Roles: []string{"customer"}, ClientID: "customer", SessionID: sid, AccountVersion: 2}, 60)
	req.Header.Set("Authorization", "Bearer "+fresh)
	if _, _, err = currentHumanClaims(sc, req); err != nil {
		t.Fatal(err)
	}
	if err = identity.RevokeSession(ctx, store, sid); err != nil {
		t.Fatal(err)
	}
	if _, _, err = currentHumanClaims(sc, req); !errors.Is(err, common.ErrSessionRevoked) {
		t.Fatal("revoked SID acquired a live identity", err)
	}
	unavailable := *sc
	unavailable.SessionRedis = redis.MustNewRedis(redis.RedisConf{Host: "127.0.0.1:63328", Type: "node", NonBlock: true})
	if _, _, err = currentHumanClaims(&unavailable, req); !errors.Is(err, common.ErrDependencyUnavailable) {
		t.Fatal("Redis outage treated as authority to access or a ban", err)
	}
}
