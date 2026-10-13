package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegionManagementRejectsOtherRoles(t *testing.T) {
	for _, role := range []string{"", "customer", "temple_admin", "master", "shop_admin", "platform_super"} {
		called := false
		h := regionAdminOnly(func(w http.ResponseWriter, r *http.Request) { called = true })
		req := httptest.NewRequest("PUT", "/api/v1/admin/platform/regions/mainland", nil)
		req.Header.Set("X-User-Id", "1")
		req.Header.Set("X-User-Roles", role)
		h(httptest.NewRecorder(), req)
		if called != (role == "platform_super") {
			t.Fatalf("incorrect access for %q", role)
		}
	}
}
