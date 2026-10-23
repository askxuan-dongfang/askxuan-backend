package logic

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/askxuan/auth-service/internal/model"
	"github.com/askxuan/auth-service/internal/types"
	"github.com/askxuan/common"
	"github.com/askxuan/common/identity"
)

func TestRetiredTempleRolesDenyLoginRefreshAndAssignmentWithoutChangingHistory(t *testing.T) {
	password, err := identity.HashPassword("fixture-password")
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"temple_reception", "temple_operations", "temple_finance"} {
		t.Run(role, func(t *testing.T) {
			f := newRefreshFixture()
			f.roles.role.Code = role
			f.roles.role.AuthzScope = "platform"
			f.admins.account.Password = password
			f.admins.account.TempleId = "T-retained"
			beforeAccount, beforeRole := *f.admins.account, *f.roles.role
			ctx := context.Background()
			if _, e := NewAdminLoginLogic(ctx, f.sc).AdminLogin(&types.AdminLoginReq{Account: "fixture-admin", Password: "fixture-password"}); !errors.Is(e, common.ErrRoleForbidden) {
				t.Fatal("retired role logged in", e)
			}
			if _, e := runFixtureRefresh(f, refreshFixtureToken(t, "admin")); !errors.Is(e, common.ErrSessionRevoked) {
				t.Fatal("retired database role refreshed a legacy token", e)
			}
			for _, roles := range [][]string{nil, {role}, {"temple_admin"}} {
				c := &common.CustomClaims{UserId: 7, UserType: "admin", Roles: roles}
				if _, e := CurrentAdminAccess(ctx, f.sc, c); !errors.Is(e, common.ErrSessionRevoked) {
					t.Fatal("retired database identity passed introspection", roles, e)
				}
			}
			if _, e := validateAdminAccountBinding(ctx, f.sc, f.roles.role.Id, "T-retained", "", 0); !errors.Is(e, common.ErrRoleForbidden) {
				t.Fatal("retired role assigned", e)
			}
			if _, e := NewAdminAccountStatusLogic(ctx, f.sc).AdminAccountStatus(&types.AdminAccountStatusReq{Id: 7, Status: "enabled"}); !errors.Is(e, common.ErrRoleForbidden) {
				t.Fatal("retired account enabled", e)
			}
			if _, e := authorizationSnapshot(ctx, f.sc, f.admins.account, f.roles.role); !errors.Is(e, common.ErrRoleForbidden) {
				t.Fatal("retired grants generated", e)
			}
			if refreshRoleMatchesDomain("admin", role) || roleCodeToClientID(role) != "" {
				t.Fatal("retired role retained a login surface")
			}
			if !reflect.DeepEqual(beforeAccount, *f.admins.account) || !reflect.DeepEqual(beforeRole, *f.roles.role) {
				t.Fatal("retirement changed historical records")
			}
		})
	}
}

type listedRoles struct {
	model.RoleModel
	rows []*model.Role
}

func (m *listedRoles) AllRoles(context.Context) ([]*model.Role, error) { return m.rows, nil }
func TestRetiredRolesAreNotSelectableButPlatformCustomRolesRemain(t *testing.T) {
	f := newRefreshFixture()
	f.sc.RoleModel = &listedRoles{rows: []*model.Role{{Id: 1, Code: "temple_admin"}, {Id: 2, Code: "master"}, {Id: 3, Code: "custom_editor", AuthzScope: "platform"}, {Id: 4, Code: "temple_reception"}, {Id: 5, Code: "temple_operations"}, {Id: 6, Code: "temple_finance"}}}
	out, e := NewAdminRoleListLogic(context.Background(), f.sc).AdminRoleList()
	if e != nil {
		t.Fatal(e)
	}
	codes := []string{}
	for _, r := range out.List {
		codes = append(codes, r.Code)
	}
	if !reflect.DeepEqual(codes, []string{"temple_admin", "master", "custom_editor"}) || !out.List[2].Editable {
		t.Fatalf("role selection changed: %+v", out.List)
	}
}

func TestExistingTempleAdministratorAndMasterKeepLoginAndLiveAccess(t *testing.T) {
	for _, role := range []string{"temple_admin", "master"} {
		f := newRefreshFixture()
		f.roles.role.Code = role
		f.admins.account.TempleId = "T-retained"
		if role == "master" {
			f.admins.account.MasterId = "M-retained"
		}
		f.admins.account.Password, _ = identity.HashPassword("fixture-password")
		login, e := NewAdminLoginLogic(context.Background(), f.sc).AdminLogin(&types.AdminLoginReq{Account: "fixture-admin", Password: "fixture-password"})
		if e != nil {
			t.Fatal(role, e)
		}
		claims, e := common.ParseToken(refreshTestSecret, login.AccessToken)
		if e != nil {
			t.Fatal(e)
		}
		if _, e = CurrentAdminAccess(context.Background(), f.sc, claims); e != nil {
			t.Fatal("existing provider lost live access", role, e)
		}
		if _, e = runFixtureRefresh(f, login.RefreshToken); e != nil {
			t.Fatal("existing provider cannot refresh", role, e)
		}
	}
}
