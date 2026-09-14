package handler

import (
	"context"
	"github.com/askxuan/common"
	"github.com/askxuan/common/identity"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"net/http/httptest"
	"os"
	"testing"
)

func TestTrustedProxyIP(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "203.0.113.7:9000"
	req.Header.Set("X-Forwarded-For", "198.51.100.2")
	t.Setenv("AUTH_TRUSTED_PROXY_CIDRS", "127.0.0.1/32,10.0.0.0/24")
	if remoteIP(req) != "203.0.113.7" {
		t.Fatal("untrusted caller forged IP")
	}
	req.RemoteAddr = "127.0.0.1:9000"
	req.Header.Set("X-Forwarded-For", "192.0.2.9, 203.0.113.7, 10.0.0.2")
	if remoteIP(req) != "203.0.113.7" {
		t.Fatal("trusted chain selected spoofed leftmost")
	}
	req.Header.Set("X-Forwarded-For", "malformed")
	if remoteIP(req) != "127.0.0.1" {
		t.Fatal("malformed proxy header")
	}
}

func TestIMTokenRejectsRevokedSession(t *testing.T) {
	host := os.Getenv("IDENTITY_TEST_REDIS")
	if host == "" {
		t.Skip("isolated Redis required")
	}
	r := redis.MustNewRedis(redis.RedisConf{Host: host, Type: "node"})
	ctx := context.Background()
	sid, e := identity.CreateSession(ctx, r, "user", 77201, 60)
	if e != nil {
		t.Fatal(e)
	}
	secret := "im-session-fixture"
	token, _ := common.GenAccessToken(secret, common.TokenInfo{UserId: 77201, UserType: "user", SessionID: sid}, 60)
	req := httptest.NewRequest("POST", "/api/v1/auth/im-token", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	if _, e = authenticatedIMIdentity(req, secret, r); e != nil {
		t.Fatal(e)
	}
	identity.RevokeSession(ctx, r, sid)
	if _, e = authenticatedIMIdentity(req, secret, r); e == nil {
		t.Fatal("revoked session obtained IM identity")
	}
}
