package middleware

import (
	"testing"

	"github.com/askxuan/common"
)

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
			claims := &common.CustomClaims{Roles: []string{tt.role}}
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
