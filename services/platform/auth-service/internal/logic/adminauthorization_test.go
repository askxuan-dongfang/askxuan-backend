package logic

import (
	"context"
	"errors"
	"github.com/askxuan/auth-service/internal/model"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"reflect"
	"testing"
)

type permissionFixtureDB struct {
	sqlx.SqlConn
	codes []string
}

func (d *permissionFixtureDB) QueryRowsCtx(_ context.Context, dest any, _ string, _ ...any) error {
	value := reflect.ValueOf(dest).Elem()
	for _, code := range d.codes {
		row := reflect.New(value.Type().Elem()).Elem()
		row.FieldByName("Code").SetString(code)
		value.Set(reflect.Append(value, row))
	}
	return nil
}
func TestAdminAuthorizationRevisionAndRefreshRevocation(t *testing.T) {
	f := newRefreshFixture()
	f.roles.role = &model.Role{Id: 20, Code: "custom_editor", AuthzScope: "platform", AuthzVersion: 3}
	f.admins.account.RoleId = 20
	f.admins.account.AuthzVersion = 4
	f.sc.DB = &permissionFixtureDB{codes: []string{"platform.products.read", "temple:update", "super.security"}}
	info := common.TokenInfo{UserId: 7, UserType: "admin", RoleID: 20, AccountVersion: 4, RoleVersion: 3, AuthzScope: "platform", Roles: []string{"custom_editor"}}
	access, _ := common.GenAccessToken(refreshTestSecret, info, 60)
	claims, _ := common.ParseToken(refreshTestSecret, access)
	out, err := CurrentAdminAccess(context.Background(), f.sc, claims)
	if err != nil || !reflect.DeepEqual(out.Permissions, []string{"platform.products.read"}) {
		t.Fatalf("live grant=%+v err=%v", out, err)
	}
	refresh, _ := common.GenRefreshToken(refreshTestSecret, info, 60)
	if _, err := runFixtureRefresh(f, refresh); err != nil {
		t.Fatal(err)
	}
	for _, change := range []string{"account", "role", "roleId", "scope", "disabled", "platformDisabled"} {
		t.Run(change, func(t *testing.T) {
			oldA, oldR := *f.admins.account, *f.roles.role
			switch change {
			case "account":
				f.admins.account.AuthzVersion++
			case "role":
				f.roles.role.AuthzVersion++
			case "roleId":
				f.admins.account.RoleId++
			case "scope":
				f.roles.role.AuthzScope = "temple"
			case "platformDisabled":
				f.admins.account.PlatformDisabled = true
			case "disabled":
				f.admins.account.Status = model.AccountStatusDisabled
			}
			if _, err := CurrentAdminAccess(context.Background(), f.sc, claims); !errors.Is(err, common.ErrSessionRevoked) {
				t.Fatal("stale access must return 40105", err)
			}
			if _, err := runFixtureRefresh(f, refresh); !errors.Is(err, common.ErrSessionRevoked) {
				t.Fatal("stale refresh must return 40105", err)
			}
			*f.admins.account = oldA
			*f.roles.role = oldR
		})
	}
}
func TestLegacyBuiltinCompatibilityAndCustomCannotUseLegacyToken(t *testing.T) {
	a := &model.AdminAccount{RoleId: 1}
	r := &model.Role{Id: 1, Code: "platform_super"}
	if !authorizationVersionMatches(&common.CustomClaims{Roles: []string{"platform_super"}}, a, r) {
		t.Fatal("legacy version-zero built-in rejected")
	}
	if authorizationVersionMatches(&common.CustomClaims{Roles: []string{"shop_admin"}}, a, r) {
		t.Fatal("legacy role mismatch accepted")
	}
	r.Code = "custom_reader"
	r.AuthzScope = "platform"
	if authorizationVersionMatches(&common.CustomClaims{}, a, r) {
		t.Fatal("custom legacy token accepted")
	}
}
func TestRoleGrantValidation(t *testing.T) {
	for _, codes := range [][]string{{"super.security"}, {"temple:update"}, {"platform.products.read", "platform.products.read"}, {"platform.products.write"}, {"does.not.exist"}} {
		if _, err := ValidatePermissionCodes(codes); err == nil {
			t.Fatalf("accepted invalid grant %v", codes)
		}
	}
	if _, err := ValidatePermissionCodes([]string{"platform.products.write", "platform.products.read"}); err != nil {
		t.Fatal(err)
	}
	if out, err := ValidatePermissionCodes(nil); err != nil || len(out) != 0 {
		t.Fatal("empty role must be valid and deny all")
	}
}
