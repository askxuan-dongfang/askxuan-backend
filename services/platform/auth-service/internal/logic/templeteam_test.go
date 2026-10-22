package logic

import (
	"github.com/askxuan/common"
	"github.com/askxuan/common/adminpolicy"
	"testing"
)

func TestTempleJobTemplateValidation(t *testing.T) {
	base := TempleTeamInput{Account: "staff.01", Password: "LongRandomPassword12", Name: "接待人员", Role: "temple_reception", Status: "enabled"}
	if !validTeamInput(base) {
		t.Fatal("valid member")
	}
	for _, role := range []string{"temple_admin", "platform_super", "master", "shop_admin", "custom"} {
		v := base
		v.Role = role
		if validTeamInput(v) {
			t.Fatalf("owner elevated role %s", role)
		}
	}
	v := base
	v.Password = "short"
	if validTeamInput(v) {
		t.Fatal("weak initial password")
	}
	for _, role := range []string{"temple_reception", "temple_operations", "temple_finance"} {
		if !adminpolicy.Builtin(role) || effectiveRoleScopeForTest(role) != "temple" || roleCodeToClientID(role) != "temple-admin" || !refreshRoleMatchesDomain("admin", role) {
			t.Fatal("identity domain mismatch", role)
		}
	}
}
func effectiveRoleScopeForTest(role string) string { return adminpolicy.Scope(role) }
func TestSignedTempleJobScope(t *testing.T) {
	c := &common.CustomClaims{UserType: "admin", ClientID: "temple-admin", AuthzScope: "temple", TempleID: 1, TempleCode: "T_OPS", Roles: []string{"temple_finance"}, Permissions: adminpolicy.TemplePermissions("temple_finance")}
	if !c.TempleStaffAllowed("GET", "/api/v1/admin/bookings/report") {
		t.Fatal("finance read denied")
	}
	c.TempleCode = ""
	if c.TempleStaffAllowed("GET", "/api/v1/admin/bookings/report") {
		t.Fatal("unbound finance allowed")
	}
}
