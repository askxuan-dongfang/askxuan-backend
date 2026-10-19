package middleware

import (
	"github.com/askxuan/common"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGrowthIdentityIsolation(t *testing.T) {
	for _, c := range []struct {
		name, kind     string
		roles          []string
		master, temple int64
		code           string
		want           [3]bool
	}{
		{"customer", "user", []string{"customer"}, 0, 0, "", [3]bool{true, false, false}},
		{"colliding admin id", "admin", []string{"customer"}, 0, 0, "", [3]bool{}},
		{"customer spoofing admin role", "user", []string{"platform_super"}, 0, 0, "", [3]bool{}},
		{"service credential", "service", []string{"platform_super"}, 0, 0, "", [3]bool{}},
		{"real operator", "admin", []string{"platform_super"}, 0, 0, "", [3]bool{false, true, false}},
		{"unlinked master", "admin", []string{"master"}, 0, 0, "", [3]bool{}},
		{"linked master", "admin", []string{"master"}, 1, 0, "", [3]bool{false, false, true}},
		{"temple missing code", "admin", []string{"temple_admin"}, 0, 1, "", [3]bool{}},
		{"linked temple", "admin", []string{"temple_admin"}, 0, 1, "T001", [3]bool{false, false, true}},
		{"shop admin", "admin", []string{"shop_admin"}, 0, 0, "", [3]bool{}},
	} {
		t.Run(c.name, func(t *testing.T) {
			claims := &common.CustomClaims{UserType: c.kind, Roles: c.roles, MasterID: c.master, TempleID: c.temple, TempleCode: c.code}
			for i, path := range []string{"overview", "admin/dashboard", "provider/bills"} {
				if got := growthRoleAllowed("/api/v1/payments/growth/"+path, claims); got != c.want[i] {
					t.Fatalf("%s: %v", path, got)
				}
			}
		})
	}
}

func TestGrowthPublicRulesReadBoundary(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		allowed      bool
	}{
		{"GET", "/api/v1/payments/growth/public/rules", true},
		{"POST", "/api/v1/payments/growth/public/rules", false},
		{"GET", "/api/v1/payments/growth/public/rules/extra", false},
		{"GET", "/api/v1/payments/growth/admin/rules", false},
		{"GET", "/api/v1/payments/growth/overview", false},
		{"POST", "/api/v1/payments/growth/catalog/preview", false},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			called := false
			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("X-User-Id", "999")
			Auth("public-rules-test", nil)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				called = true
				if r.Header.Get("X-User-Id") != "" {
					t.Fatal("guest identity header retained")
				}
			})).ServeHTTP(httptest.NewRecorder(), req)
			if called != tc.allowed {
				t.Fatalf("allowed=%v want=%v", called, tc.allowed)
			}
		})
	}
}
