package adminpolicy

import "testing"

func TestTempleJobsDenyUnknownAndPrivilegedRoutes(t *testing.T) {
	for _, role := range []string{"temple_reception", "temple_operations", "temple_finance"} {
		perms := append(TemplePermissions(role), "super.security", "temple.team.manage", "temple.finance.read")
		for _, r := range []Route{{"POST", "/api/v1/auth/temple/team", ""}, {"POST", "/api/v1/admin/auth/accounts", ""}, {"GET", "/api/v1/admin/masters/profile", ""}, {"POST", "/api/v1/finance/wallet/temple/withdraw", ""}, {"PUT", "/api/v1/admin/temples/new-api", ""}} {
			if TempleAllowed(r.Method, r.Template, role, perms) {
				t.Errorf("%s escaped on %s", role, r.Template)
			}
		}
	}
	if TempleAllowed("GET", "/api/v1/admin/bookings/report", "temple_reception", TemplePermissions("temple_reception")) {
		t.Fatal("report mistaken for booking detail")
	}
	if !TempleAllowed("PUT", "/api/v1/admin/bookings/B1/confirm", "temple_reception", TemplePermissions("temple_reception")) {
		t.Fatal("reception cannot confirm")
	}
	if TempleAllowed("PUT", "/api/v1/admin/bookings/B1/confirm", "temple_finance", TemplePermissions("temple_finance")) {
		t.Fatal("finance can write")
	}
	if !TempleAllowed("GET", "/api/v1/admin/bookings/report", "temple_finance", TemplePermissions("temple_finance")) {
		t.Fatal("finance cannot read report")
	}
	if TempleAllowed("PUT", "/api/v1/admin/temples/schedules", "temple_operations", nil) {
		t.Fatal("unsigned permission")
	}
}
