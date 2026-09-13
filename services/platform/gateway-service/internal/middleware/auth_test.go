package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/askxuan/common"
	"github.com/golang-jwt/jwt/v5"
)

func TestMarketingActivityGuestReadBoundary(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		public       bool
	}{
		{http.MethodGet, "/api/v1/marketing/activities", true},
		{http.MethodGet, "/api/v1/marketing/activities/1", true},
		{http.MethodPost, "/api/v1/marketing/activities", false},
		{http.MethodPut, "/api/v1/marketing/activities/1", false},
		{http.MethodGet, "/api/v1/marketing/activities-private", false},
		{http.MethodGet, "/api/v1/admin/marketing/activities", false},
		{http.MethodPut, "/api/v1/admin/marketing/banners/1", false},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			called := false
			handler := Auth("local-test-secret", nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(tc.method, tc.path, nil))
			if called != tc.public {
				t.Fatalf("reached service=%v, expected=%v", called, tc.public)
			}
		})
	}
}

func TestRoleAllowedForAdminPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		role    string
		allowed bool
	}{
		{name: "platform finance", path: "/api/v1/admin/finance/overview", role: "platform_super", allowed: true},
		{name: "temple cannot read finance", path: "/api/v1/admin/finance/overview", role: "temple_admin", allowed: false},
		{name: "master withdrawal", path: "/api/v1/admin/finance/withdrawals/apply", role: "master", allowed: true},
		{name: "master cannot audit withdrawals", path: "/api/v1/admin/finance/withdrawals/1/audit", role: "master", allowed: false},
		{name: "temple booking", path: "/api/v1/admin/bookings", role: "temple_admin", allowed: true},
		{name: "shop cannot read temple booking", path: "/api/v1/admin/bookings", role: "shop_admin", allowed: false},
		{name: "shop points", path: "/api/v1/admin/points/products", role: "shop_admin", allowed: true},
		{name: "temple cannot operate points", path: "/api/v1/admin/points/orders", role: "temple_admin", allowed: false},
		{name: "master cannot operate points", path: "/api/v1/admin/points/report", role: "master", allowed: false},
		{name: "customer cannot operate points", path: "/api/v1/admin/points/products", role: "customer", allowed: false},
		{name: "platform points", path: "/api/v1/admin/points/report", role: "platform_super", allowed: true},
		{name: "shop products", path: "/api/v1/admin/products", role: "shop_admin", allowed: true},
		{name: "master community", path: "/api/v1/admin/masters/community/posts", role: "master", allowed: true},
		{name: "temple cannot publish as master", path: "/api/v1/admin/masters/community/posts", role: "temple_admin", allowed: false},
		{name: "shop cannot configure rewards", path: "/api/v1/admin/marketing/rewards/campaigns", role: "shop_admin", allowed: false},
		{name: "platform rewards", path: "/api/v1/admin/marketing/rewards/campaigns", role: "platform_super", allowed: true},
		{name: "shop cannot configure accounts", path: "/api/v1/admin/auth/accounts", role: "shop_admin", allowed: false},
		{name: "shop logistics preserved", path: "/api/v1/admin/logistics/express", role: "shop_admin", allowed: true},
		{name: "platform override", path: "/api/v1/admin/products", role: "platform_super", allowed: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claims := &common.CustomClaims{UserType: "admin", Roles: []string{tt.role}}
			if got := roleAllowedForAdminPath(tt.path, claims); got != tt.allowed {
				t.Fatalf("role=%s path=%s got=%v want=%v", tt.role, tt.path, got, tt.allowed)
			}
		})
	}
}

func TestServiceTokenCanCallAdminIntegrations(t *testing.T) {
	claims := &common.CustomClaims{UserType: "service", Roles: []string{"platform_service"}}
	if !roleAllowedForAdminPath("/api/v1/admin/audit/queue", claims) {
		t.Fatal("platform service token should retain integration access")
	}
}

func TestProviderSettingsRequireRealSuperAdminClaims(t *testing.T) {
	for _, tc := range []struct {
		kind, role string
		allowed    bool
	}{{"user", "customer", false}, {"admin", "shop_admin", false}, {"admin", "platform_service", false}, {"service", "platform_service", false}, {"service", "platform_super", false}, {"admin", "platform_super", true}} {
		token, err := common.GenAccessToken("settings-test-secret", common.TokenInfo{UserId: 1, UserType: tc.kind, Roles: []string{tc.role}}, 60)
		if err != nil {
			t.Fatal(err)
		}
		for _, method := range []string{"GET", "PUT", "POST"} {
			called := false
			next := Auth("settings-test-secret", nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))
			r := httptest.NewRequest(method, "/api/v1/ai/admin/provider", nil)
			r.Header.Set("Authorization", "Bearer "+token)
			r.Header.Set("X-User-Type", "admin")
			r.Header.Set("X-User-Roles", "platform_super")
			next.ServeHTTP(httptest.NewRecorder(), r)
			if called != tc.allowed {
				t.Fatalf("%s %s/%s allowed=%v", method, tc.kind, tc.role, called)
			}
		}
	}
}

func TestMessageAdminGatewayAuthorization(t *testing.T) {
	const secret = "message-gateway-test-secret"
	for _, identity := range []struct {
		name, kind, role string
		masterID         int64
		refresh          bool
		platform, master bool
	}{
		{name: "anonymous"},
		{name: "customer", kind: "user", role: "customer"},
		{name: "shop", kind: "admin", role: "shop_admin"},
		{name: "temple", kind: "admin", role: "temple_admin"},
		{name: "master", kind: "admin", role: "master", masterID: 41, master: true},
		{name: "master without identity", kind: "admin", role: "master"},
		{name: "human service role", kind: "admin", role: "platform_service"},
		{name: "internal service", kind: "service", role: "platform_service"},
		{name: "service super role", kind: "service", role: "platform_super"},
		{name: "customer super role", kind: "user", role: "platform_super"},
		{name: "super without identity type", role: "platform_super"},
		{name: "super refresh", kind: "admin", role: "platform_super", refresh: true},
		{name: "master refresh", kind: "admin", role: "master", masterID: 41, refresh: true},
		{name: "platform super", kind: "admin", role: "platform_super", platform: true},
	} {
		for _, route := range []struct {
			method, path string
			master       bool
		}{
			{http.MethodPost, "/api/v1/admin/messages/push", false},
			{http.MethodGet, "/api/v1/admin/messages/push-logs", false},
			{http.MethodGet, "/api/v1/admin/messages/templates", false},
			{http.MethodPost, "/api/v1/admin/messages/templates", false},
			{http.MethodPut, "/api/v1/admin/messages/templates/1", false},
			{http.MethodGet, "/api/v1/admin/messages/master", true},
			{http.MethodPut, "/api/v1/admin/messages/master/1/read", true},
			{http.MethodGet, "/api/v1/admin/messages/master-private", false},
			{http.MethodPost, "/api/v1/admin/announcements/create", false},
		} {
			t.Run(identity.name+"/"+route.method+route.path, func(t *testing.T) {
				req := httptest.NewRequest(route.method, route.path, nil)
				if identity.name != "anonymous" {
					claims := common.CustomClaims{UserId: 7, UserType: identity.kind, Roles: []string{identity.role}, MasterID: identity.masterID, Type: "access", RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
					if identity.refresh {
						claims.Type = "refresh"
					}
					token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
					if err != nil {
						t.Fatal(err)
					}
					req.Header.Set("Authorization", "Bearer "+token)
				}
				req.Header.Set("X-User-Id", "999")
				req.Header.Set("X-User-Type", "admin")
				req.Header.Set("X-User-Roles", "platform_super,master")
				req.Header.Set("X-Master-Id", "999")
				called := false
				rr := httptest.NewRecorder()
				Auth(secret, nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })).ServeHTTP(rr, req)
				want := identity.platform
				if route.master {
					want = identity.master
				}
				if called != want {
					t.Fatalf("reached downstream=%v want=%v body=%s", called, want, rr.Body.String())
				}
				if !want {
					var body common.Body
					if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil || body.Code == 0 {
						t.Fatalf("expected authorization error: %v %s", err, rr.Body.String())
					}
				}
			})
		}
	}
}

func TestAdminRolesCannotSubstituteIdentityDomain(t *testing.T) {
	for _, path := range []string{
		"/api/v1/admin/products", "/api/v1/admin/orders", "/api/v1/admin/diy/orders",
		"/api/v1/admin/points/products", "/api/v1/admin/marketing/coupons", "/api/v1/admin/auth/accounts",
	} {
		for _, kind := range []string{"", "user", "service", "admin"} {
			t.Run(path+"/kind="+kind, func(t *testing.T) {
				token, err := common.GenAccessToken("identity-boundary-test", common.TokenInfo{UserId: 7, UserType: kind, Roles: []string{"platform_super"}}, 60)
				if err != nil {
					t.Fatal(err)
				}
				req := httptest.NewRequest(http.MethodPost, path, nil)
				req.Header.Set("Authorization", "Bearer "+token)
				req.Header.Set("X-User-Type", "admin")
				req.Header.Set("X-User-Roles", "platform_super")
				called := false
				rr := httptest.NewRecorder()
				Auth("identity-boundary-test", nil)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })).ServeHTTP(rr, req)
				if called != (kind == "admin") {
					t.Fatalf("kind=%q reached service=%v body=%s", kind, called, rr.Body.String())
				}
			})
		}
	}
}
