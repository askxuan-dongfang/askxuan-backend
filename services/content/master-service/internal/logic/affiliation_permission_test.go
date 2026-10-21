package logic

import (
	"context"
	"testing"

	"github.com/askxuan/common/middleware"
	"github.com/askxuan/master-service/internal/types"
)

func TestAffiliationUpdatePrivilege(t *testing.T) {
	if _, err := NewWorkspaceProfileUpdateLogic(context.Background(), nil).WorkspaceProfileUpdate(&types.MasterProfileUpdateReq{TraditionCodes: []string{"chan"}}); err == nil {
		t.Fatal("master self-service may not change standard affiliations")
	}
	temple := context.WithValue(context.Background(), middleware.CtxKeyTempleCode, "T001")
	if _, err := NewAdminMasterUpdateLogic(temple, nil).AdminMasterUpdate(&types.AdminMasterUpdateReq{TraditionCodes: []string{"chan"}}, "T001"); err == nil {
		t.Fatal("temple may not change existing managed affiliations")
	}
	for _, role := range []string{"platform_admin", "temple_admin", "master", "master_applicant", "temple_applicant", ""} {
		ctx := context.WithValue(context.Background(), middleware.CtxKeyRoles, []string{role})
		if canEditMasterAffiliation(ctx) {
			t.Fatalf("unexpected privilege %q", role)
		}
		if _, err := NewPlatformMasterUpdateLogic(ctx, nil).PlatformMasterUpdate(&types.PlatformMasterUpdateReq{TraditionCodes: []string{"chan"}}); err == nil {
			t.Fatalf("relation update allowed for %q", role)
		}
	}
	if !canEditMasterAffiliation(context.WithValue(context.Background(), middleware.CtxKeyRoles, []string{"platform_super"})) {
		t.Fatal("super privilege missing")
	}
}
