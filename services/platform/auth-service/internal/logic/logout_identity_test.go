package logic

import (
	"context"
	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/auth-service/internal/types"
	"github.com/askxuan/common"
	"github.com/askxuan/common/identity"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"os"
	"testing"
)

func TestExpiredTokenRevocation(t *testing.T) {
	secret := "logout-fixture-only"
	token, _ := common.GenAccessToken(secret, common.TokenInfo{SessionID: "fixture", UserId: 1, UserType: "user"}, -60)
	if _, e := common.ParseToken(secret, token); e == nil {
		t.Fatal("expired token authenticated")
	}
	if c, e := revocationClaims(secret, token); e != nil || c.SessionID != "fixture" {
		t.Fatal("expired token cannot revoke", e)
	}
	if _, e := revocationClaims("wrong-key", token); e == nil {
		t.Fatal("forged token can revoke")
	}
	host := os.Getenv("IDENTITY_TEST_REDIS")
	if host == "" {
		return
	}
	r := redis.MustNewRedis(redis.RedisConf{Host: host, Type: "node"})
	ctx := context.Background()
	sid, e := identity.CreateSession(ctx, r, "user", 77101, 60)
	if e != nil {
		t.Fatal(e)
	}
	token, _ = common.GenAccessToken(secret, common.TokenInfo{SessionID: sid, UserId: 77101, UserType: "user"}, -60)
	sc := &svc.ServiceContext{SessionRedis: r, Redis: r}
	sc.Config.Auth.AccessSecret = secret
	if _, e = NewLogoutLogic(ctx, sc).Logout(&types.LogoutReq{AccessToken: token}); e != nil {
		t.Fatal(e)
	}
	if identity.CheckSession(ctx, r, sid, "user", 77101) == nil {
		t.Fatal("expired access left refresh session alive")
	}
}
