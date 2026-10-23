package adminpolicy

import "testing"

func TestTempleAdministratorReplacesRetiredJobs(t *testing.T) {
	for _, role := range []string{"temple_reception", "temple_operations", "temple_finance"} {
		if !IsRetiredTempleRole(role) || !Builtin(role) || IsTempleRole(role) || Scope(role) != "" {
			t.Fatalf("retired job must be reserved without a live institution scope: %s", role)
		}
	}
	if !IsTempleRole("temple_admin") || IsRetiredTempleRole("temple_admin") || Scope("temple_admin") != "temple" {
		t.Fatal("existing temple administrator lost its scope")
	}
	for _, role := range []string{"master", "platform_super", "custom_editor"} {
		if IsTempleRole(role) || IsRetiredTempleRole(role) {
			t.Fatal("unrelated role changed", role)
		}
	}
}
