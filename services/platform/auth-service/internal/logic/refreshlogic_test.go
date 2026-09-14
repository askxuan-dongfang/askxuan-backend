package logic

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/askxuan/auth-service/internal/model"
	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/auth-service/internal/types"
	"github.com/askxuan/common"
	"github.com/askxuan/common/identity"
	"github.com/golang-jwt/jwt/v5"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const refreshTestSecret = "isolated-refresh-fixture-key"

type refreshUsers struct {
	model.UserReadonlyModel
	user                 *model.User
	err                  error
	idReads, mobileReads int
}

func (m *refreshUsers) FindByID(context.Context, int64) (*model.User, error) {
	m.idReads++
	return m.user, m.err
}
func (m *refreshUsers) FindByMobile(context.Context, string) (*model.User, error) {
	m.mobileReads++
	return m.user, m.err
}

type refreshAdmins struct {
	model.AdminAccountModel
	account *model.AdminAccount
	err     error
	idReads int
}

func (m *refreshAdmins) FindByID(context.Context, int64) (*model.AdminAccount, error) {
	m.idReads++
	return m.account, m.err
}
func (m *refreshAdmins) FindByAccount(context.Context, string) (*model.AdminAccount, error) {
	return m.account, m.err
}
func (m *refreshAdmins) UpdateLastLogin(context.Context, int64) error { return nil }

type refreshRoles struct {
	model.RoleModel
	role *model.Role
	err  error
}

func (m *refreshRoles) FindByID(context.Context, int64) (*model.Role, error) { return m.role, m.err }

type refreshBlacklist struct {
	value string
	err   error
	reads int
}

func (b *refreshBlacklist) GetCtx(context.Context, string) (string, error) {
	b.reads++
	return b.value, b.err
}
func (*refreshBlacklist) Setex(string, string, int) error { return nil }

type refreshDB struct {
	sqlx.SqlConn
	templeStatus, masterAuth, masterPlatform, masterTemple string
	err                                                    error
}

func (d *refreshDB) QueryRowCtx(_ context.Context, dest any, query string, _ ...any) error {
	if d.err != nil {
		return d.err
	}
	switch v := dest.(type) {
	case *string:
		*v = d.templeStatus
	case *int64:
		*v = 101
	default:
		if !strings.Contains(query, "m.auth_status") {
			return errors.New("unexpected fixture query")
		}
		row := reflect.ValueOf(dest).Elem()
		row.FieldByName("AuthStatus").SetString(d.masterAuth)
		row.FieldByName("PlatformStatus").SetString(d.masterPlatform)
		row.FieldByName("TempleStatus").SetString(d.masterTemple)
	}
	return nil
}

type refreshFixture struct {
	sc        *svc.ServiceContext
	users     *refreshUsers
	admins    *refreshAdmins
	roles     *refreshRoles
	blacklist *refreshBlacklist
	db        *refreshDB
}

func newRefreshFixture() *refreshFixture {
	f := &refreshFixture{
		users:     &refreshUsers{user: &model.User{Id: 7, Mobile: "fixture-user", Password: "fixture-password", Status: 1}},
		admins:    &refreshAdmins{account: &model.AdminAccount{Id: 7, Account: "fixture-admin", Password: "fixture-password", RoleId: 1, Status: model.AccountStatusEnabled}},
		roles:     &refreshRoles{role: &model.Role{Id: 1, Code: model.RoleCodePlatformSuper}},
		blacklist: &refreshBlacklist{},
		db:        &refreshDB{templeStatus: "正常", masterAuth: "已认证", masterPlatform: "normal"},
	}
	f.sc = &svc.ServiceContext{UserReadonlyModel: f.users, AdminAccountModel: f.admins, RoleModel: f.roles, Redis: f.blacklist, DB: f.db}
	f.sc.Config.Auth.AccessSecret = refreshTestSecret
	f.sc.Config.Auth.AccessExpire = 60
	f.sc.Config.Auth.RefreshExpire = 600
	return f
}
func refreshFixtureToken(t *testing.T, domain string) string {
	t.Helper()
	token, err := common.GenRefreshToken(refreshTestSecret, common.TokenInfo{UserId: 7, UserType: domain}, 600)
	if err != nil {
		t.Fatal(err)
	}
	return token
}
func runFixtureRefresh(f *refreshFixture, token string) (*types.RefreshResp, error) {
	return NewRefreshLogic(context.Background(), f.sc).Refresh(&types.RefreshReq{RefreshToken: token})
}

func TestSignInAndRefreshKeepIdentityDomainsSeparate(t *testing.T) {
	for _, domain := range []string{"user", "admin", "master"} {
		t.Run(domain, func(t *testing.T) {
			f := newRefreshFixture() // The user and administrator intentionally have the SAME ID.
			var login *types.LoginResp
			var err error
			if domain == "user" {
				login, err = NewLoginLogic(context.Background(), f.sc).IssueCustomer(f.users.user.Id)
				f.users.idReads = 0
			} else {
				if domain == "master" {
					f.roles.role.Code = model.RoleCodeMaster
					f.admins.account.MasterId = "M-fixture"
				}
				f.admins.account.Password, _ = identity.HashPassword("fixture-password")
				login, err = NewAdminLoginLogic(context.Background(), f.sc).AdminLogin(&types.AdminLoginReq{Account: "fixture-admin", Password: "fixture-password"})
			}
			if err != nil {
				t.Fatal(err)
			}
			refreshClaims, err := common.ParseToken(refreshTestSecret, login.RefreshToken)
			if err != nil || refreshClaims.UserType != domain {
				t.Fatalf("wrong refresh domain: %+v, %v", refreshClaims, err)
			}
			f.users.user.Mobile = "updated-user" // Refresh must use the original ID, not stale contact details.
			resp, err := runFixtureRefresh(f, login.RefreshToken)
			if err != nil {
				t.Fatal(err)
			}
			claims, err := common.ParseToken(refreshTestSecret, resp.AccessToken)
			if err != nil {
				t.Fatal(err)
			}
			if domain == "user" {
				if claims.UserType != "user" || !claims.HasRole("customer") || claims.Mobile != "updated-user" || f.admins.idReads != 0 || f.users.idReads != 1 {
					t.Fatalf("user crossed identity domain: %+v", claims)
				}
			} else {
				if claims.UserType != "admin" || !claims.HasRole(f.roles.role.Code) || claims.ClientID != roleCodeToClientID(f.roles.role.Code) || f.users.idReads != 0 || f.admins.idReads != 1 {
					t.Fatalf("admin crossed identity domain: %+v", claims)
				}
				if domain == "master" && claims.MasterID != 101 {
					t.Fatalf("master binding lost: %+v", claims)
				}
			}
		})
	}
}

func TestRefreshRejectsLegacyAndInvalidIdentityBeforeLookup(t *testing.T) {
	base := common.CustomClaims{UserId: 7, UserType: "user", Type: "refresh", RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))}}
	cases := []struct {
		name   string
		mutate func(*common.CustomClaims)
	}{
		{"legacy-missing-domain", func(c *common.CustomClaims) { c.UserType = "" }},
		{"unknown-domain", func(c *common.CustomClaims) { c.UserType = "administrator" }},
		{"service-domain", func(c *common.CustomClaims) { c.UserType = "service" }},
		{"access-token", func(c *common.CustomClaims) { c.Type = "access" }},
		{"missing-expiry", func(c *common.CustomClaims) { c.ExpiresAt = nil }},
		{"invalid-id", func(c *common.CustomClaims) { c.UserId = 0 }},
		{"expired", func(c *common.CustomClaims) { c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute)) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newRefreshFixture()
			claims := base
			tc.mutate(&claims)
			token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(refreshTestSecret))
			if err != nil {
				t.Fatal(err)
			}
			if resp, err := runFixtureRefresh(f, token); !errors.Is(err, common.ErrTokenInvalid) || resp != nil {
				t.Fatalf("invalid identity accepted: %+v %v", resp, err)
			}
			if f.blacklist.reads != 0 || f.users.idReads != 0 || f.admins.idReads != 0 {
				t.Fatal("invalid identity reached storage")
			}
		})
	}
}

func TestRefreshRechecksAvailabilityAndFailsClosed(t *testing.T) {
	cases := []struct {
		name, domain string
		change       func(*refreshFixture)
		want         error
	}{
		{"user-disabled", "user", func(f *refreshFixture) { f.users.user.Status = 0 }, common.ErrTokenInvalid},
		{"user-deleted", "user", func(f *refreshFixture) { f.users.err = sqlx.ErrNotFound }, common.ErrTokenInvalid},
		{"user-missing", "user", func(f *refreshFixture) { f.users.user = nil }, common.ErrTokenInvalid},
		{"user-id-mismatch", "user", func(f *refreshFixture) { f.users.user.Id = 8 }, common.ErrTokenInvalid},
		{"user-database-error", "user", func(f *refreshFixture) { f.users.err = errors.New("db unavailable") }, common.ErrSystem},
		{"admin-disabled", "admin", func(f *refreshFixture) { f.admins.account.Status = model.AccountStatusDisabled }, common.ErrTokenInvalid},
		{"admin-deleted", "admin", func(f *refreshFixture) { f.admins.err = sqlx.ErrNotFound }, common.ErrTokenInvalid},
		{"admin-missing", "admin", func(f *refreshFixture) { f.admins.account = nil }, common.ErrTokenInvalid},
		{"admin-id-mismatch", "admin", func(f *refreshFixture) { f.admins.account.Id = 8 }, common.ErrTokenInvalid},
		{"admin-database-error", "admin", func(f *refreshFixture) { f.admins.err = errors.New("db unavailable") }, common.ErrSystem},
		{"role-deleted", "admin", func(f *refreshFixture) { f.roles.err = sqlx.ErrNotFound }, common.ErrTokenInvalid},
		{"role-missing", "admin", func(f *refreshFixture) { f.roles.role = nil }, common.ErrTokenInvalid},
		{"role-unknown", "admin", func(f *refreshFixture) { f.roles.role.Code = "admin" }, common.ErrTokenInvalid},
		{"role-database-error", "admin", func(f *refreshFixture) { f.roles.err = errors.New("db unavailable") }, common.ErrSystem},
		{"admin-became-master", "admin", func(f *refreshFixture) { f.roles.role.Code = "master" }, common.ErrTokenInvalid},
		{"master-became-shop", "master", func(f *refreshFixture) { f.roles.role.Code = "shop_admin" }, common.ErrTokenInvalid},
		{"temple-disabled", "admin", func(f *refreshFixture) {
			f.roles.role.Code = "temple_admin"
			f.admins.account.TempleId = "T-fixture"
			f.db.templeStatus = "封禁"
		}, common.ErrTokenInvalid},
		{"temple-deleted", "admin", func(f *refreshFixture) {
			f.roles.role.Code = "temple_admin"
			f.admins.account.TempleId = "T-fixture"
			f.db.err = sqlx.ErrNotFound
		}, common.ErrTokenInvalid},
		{"temple-unbound", "admin", func(f *refreshFixture) { f.roles.role.Code = "temple_admin" }, common.ErrTokenInvalid},
		{"master-disabled", "master", func(f *refreshFixture) {
			f.roles.role.Code = "master"
			f.admins.account.MasterId = "M-fixture"
			f.db.masterPlatform = "banned"
		}, common.ErrTokenInvalid},
		{"master-unverified", "master", func(f *refreshFixture) {
			f.roles.role.Code = "master"
			f.admins.account.MasterId = "M-fixture"
			f.db.masterAuth = "待审核"
		}, common.ErrTokenInvalid},
		{"master-temple-disabled", "master", func(f *refreshFixture) {
			f.roles.role.Code = "master"
			f.admins.account.MasterId = "M-fixture"
			f.db.masterTemple = "封禁"
		}, common.ErrTokenInvalid},
		{"master-deleted", "master", func(f *refreshFixture) {
			f.roles.role.Code = "master"
			f.admins.account.MasterId = "M-fixture"
			f.db.err = sqlx.ErrNotFound
		}, common.ErrTokenInvalid},
		{"master-unbound", "master", func(f *refreshFixture) { f.roles.role.Code = "master" }, common.ErrTokenInvalid},
		{"blacklisted", "admin", func(f *refreshFixture) { f.blacklist.value = "1" }, common.ErrTokenInvalid},
		{"blacklist-unavailable", "admin", func(f *refreshFixture) { f.blacklist.err = errors.New("redis unavailable") }, common.ErrSystem},
		{"blacklist-unconfigured", "admin", func(f *refreshFixture) { f.sc.Redis = nil }, common.ErrSystem},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newRefreshFixture()
			tc.change(f)
			if resp, err := runFixtureRefresh(f, refreshFixtureToken(t, tc.domain)); !errors.Is(err, tc.want) || resp != nil {
				t.Fatalf("got %+v %v; want %v", resp, err, tc.want)
			}
		})
	}
}

func TestRefreshReloadsCurrentAdminRole(t *testing.T) {
	f := newRefreshFixture()
	token := refreshFixtureToken(t, "admin")
	f.roles.role.Code = "shop_admin"
	resp, err := runFixtureRefresh(f, token)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := common.ParseToken(refreshTestSecret, resp.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserType != "admin" || !claims.HasRole("shop_admin") || claims.HasRole("platform_super") || claims.ClientID != "shop-admin" {
		t.Fatalf("stale administrator privileges: %+v", claims)
	}
}
