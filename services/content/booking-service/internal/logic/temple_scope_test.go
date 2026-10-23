package logic

import (
	"context"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"testing"
)

func TestExceptionScopeRequiresExistingTempleAdministrator(t *testing.T) {
	for _, role := range []string{"temple_admin", "temple_reception", "temple_operations", "temple_finance", "master"} {
		ctx := context.WithValue(context.Background(), middleware.CtxKeyAdminClaims, &common.CustomClaims{UserType: "admin", Roles: []string{role}, TempleCode: "T1"})
		code, e := exceptionScope(ctx, "", false)
		if role == "temple_admin" {
			if e != nil || code != "T1" {
				t.Fatal(code, e)
			}
		} else if e == nil {
			t.Fatal("retired or unrelated role reached temple exceptions", role)
		}
		if _, e = exceptionScope(ctx, "T2", false); e == nil {
			t.Fatal("cross-temple scope accepted", role)
		}
	}
}
