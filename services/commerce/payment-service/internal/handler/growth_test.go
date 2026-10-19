package handler

import (
	"context"
	m "github.com/askxuan/common/middleware"
	"net/http/httptest"
	"testing"
)

func TestGrowthServiceIdentityIsolation(t *testing.T) {
	for _, c := range []struct {
		kind, role, surface string
		allowed             bool
	}{
		{"user", "customer", "customer", true}, {"admin", "customer", "customer", false}, {"user", "platform_super", "admin", false}, {"service", "platform_super", "admin", false}, {"admin", "platform_super", "admin", true}, {"admin", "shop_admin", "admin", false}, {"admin", "master", "provider", false},
	} {
		ctx := context.WithValue(context.Background(), m.CtxKeyUserID, int64(7))
		ctx = context.WithValue(ctx, m.CtxKeyUserType, c.kind)
		ctx = context.WithValue(ctx, m.CtxKeyRoles, []string{c.role})
		if growthIdentity(httptest.NewRequest("GET", "/", nil).WithContext(ctx), c.surface) != c.allowed {
			t.Fatal(c)
		}
	}
}
