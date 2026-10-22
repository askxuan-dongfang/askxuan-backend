package middleware

import (
	"github.com/askxuan/common"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDelegationRequiresVerifiedSignedScopeAndExactCapability(t *testing.T) {
	for _, tc := range []struct {
		name, kind, role, scope, client, method, path string
		perms                                         []string
		allowed                                       bool
	}{
		{"read", "admin", "content_reader", "platform", "platform-admin", "GET", "/api/v1/admin/products", []string{"platform.products.read"}, true},
		{"write", "admin", "content_editor", "platform", "platform-admin", "POST", "/api/v1/admin/products", []string{"platform.products.write"}, true},
		{"read cannot write", "admin", "content_reader", "platform", "platform-admin", "POST", "/api/v1/admin/products", []string{"platform.products.read"}, false},
		{"wrong client", "admin", "content_reader", "platform", "temple-admin", "GET", "/api/v1/admin/products", []string{"platform.products.read"}, false},
		{"wrong scope", "admin", "content_reader", "temple", "platform-admin", "GET", "/api/v1/admin/products", []string{"platform.products.read"}, false},
		{"customer", "user", "content_reader", "platform", "platform-admin", "GET", "/api/v1/admin/products", []string{"platform.products.read"}, false},
		{"service", "service", "content_reader", "platform", "platform-admin", "GET", "/api/v1/admin/products", []string{"platform.products.read"}, false},
		{"provider cannot acquire platform", "admin", "temple_admin", "platform", "platform-admin", "GET", "/api/v1/admin/products", []string{"platform.products.read"}, false},
		{"provider path", "admin", "content_editor", "platform", "platform-admin", "PUT", "/api/v1/admin/masters/profile", []string{"platform.masters.write"}, false},
		{"locked root", "admin", "content_editor", "platform", "platform-admin", "POST", "/api/v1/admin/diy/creator-withdrawals/1/action", []string{"platform.diy.write", "super.creator"}, false},
		{"forged header", "admin", "content_reader", "", "platform-admin", "GET", "/api/v1/admin/products", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			token, err := common.GenAccessToken("test-permissions-secret", common.TokenInfo{UserId: 7, UserType: tc.kind, Roles: []string{tc.role}, AuthzScope: tc.scope, ClientID: tc.client, Permissions: tc.perms}, 60)
			if err != nil {
				t.Fatal(err)
			}
			called := false
			handler := (&AuthConfig{Secret: "test-permissions-secret"}).Auth((&AdminAuthConfig{AllowedRoles: []string{"platform_super", "shop_admin"}}).AdminAuth(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })))
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("X-User-Permissions", "platform.products.read,platform.products.write")
			req.Header.Set("X-Authz-Scope", "platform")
			handler.ServeHTTP(httptest.NewRecorder(), req)
			if called != tc.allowed {
				t.Fatalf("called=%v want=%v", called, tc.allowed)
			}
		})
	}
}
