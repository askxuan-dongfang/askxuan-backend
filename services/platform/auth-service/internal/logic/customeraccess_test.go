package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/askxuan/auth-service/internal/types"
	"github.com/askxuan/common"
	"github.com/askxuan/common/identity"
)

func TestCustomerBanAndRestorationNeverResurrectOldCredentials(t *testing.T) {
	f := newRefreshFixture()
	old, err := NewLoginLogic(context.Background(), f.sc).IssueCustomer(7)
	if err != nil {
		t.Fatal(err)
	}
	oldAccess, err := common.ParseToken(refreshTestSecret, old.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = CurrentCustomerAccess(context.Background(), f.sc, oldAccess); err != nil {
		t.Fatal(err)
	}
	// Status and revision are changed by a single database statement. This models
	// the snapshots a request can observe, including a concurrent delayed login.
	for _, state := range []struct {
		status   int
		revision int64
	}{{0, 1}, {1, 2}} {
		f.users.user.Status, f.users.user.AuthzVersion = state.status, state.revision
		if _, err = CurrentCustomerAccess(context.Background(), f.sc, oldAccess); !errors.Is(err, common.ErrSessionRevoked) {
			t.Fatalf("old access accepted at %+v: %v", state, err)
		}
		if _, err = NewRefreshLogic(context.Background(), f.sc).Refresh(&types.RefreshReq{RefreshToken: old.RefreshToken}); !errors.Is(err, common.ErrSessionRevoked) {
			t.Fatalf("old refresh accepted at %+v: %v", state, err)
		}
	}
	fresh, err := NewLoginLogic(context.Background(), f.sc).IssueCustomer(7)
	if err != nil {
		t.Fatal(err)
	}
	freshAccess, _ := common.ParseToken(refreshTestSecret, fresh.AccessToken)
	freshRefresh, _ := common.ParseToken(refreshTestSecret, fresh.RefreshToken)
	if freshAccess.AccountVersion != 2 || freshRefresh.AccountVersion != 2 {
		t.Fatal("new login dropped durable revision")
	}
	if _, err = CurrentCustomerAccess(context.Background(), f.sc, freshAccess); err != nil {
		t.Fatal("new login after restoration rejected", err)
	}
	if _, err = NewRefreshLogic(context.Background(), f.sc).Refresh(&types.RefreshReq{RefreshToken: fresh.RefreshToken}); err != nil {
		t.Fatal(err)
	}
	if f.admins.idReads != 0 {
		t.Fatal("consumer status consulted the admin identity with the same ID")
	}
}

func TestCustomerAccessReloadsEveryTimeAndRejectsUnknownDomainOrAvailability(t *testing.T) {
	f := newRefreshFixture()
	c := common.CustomClaims{UserId: 7, UserType: "user", Roles: []string{"customer"}, ClientID: "customer"}
	for n := 0; n < 3; n++ {
		if _, err := CurrentCustomerAccess(context.Background(), f.sc, &c); err != nil {
			t.Fatal(err)
		}
	}
	if f.users.idReads != 3 {
		t.Fatal("availability was cached")
	}
	f.users.err = errors.New("database unavailable")
	if _, err := CurrentCustomerAccess(context.Background(), f.sc, &c); !errors.Is(err, common.ErrDependencyUnavailable) {
		t.Fatal("DB outage opened access", err)
	}
	f.users.err = nil
	for _, mutate := range []func(*common.CustomClaims){
		func(c *common.CustomClaims) { c.UserType = "admin" },
		func(c *common.CustomClaims) { c.Type = "refresh" },
		func(c *common.CustomClaims) { c.Roles = []string{"platform_super"} },
		func(c *common.CustomClaims) { c.ClientID = "master" },
		func(c *common.CustomClaims) { c.UserId = 0 },
	} {
		bad := c
		mutate(&bad)
		if _, err := CurrentCustomerAccess(context.Background(), f.sc, &bad); !errors.Is(err, common.ErrTokenInvalid) {
			t.Fatal("invalid consumer domain accepted", err)
		}
	}
	f.users.user.Status = 9
	if _, err := CurrentCustomerAccess(context.Background(), f.sc, &c); !errors.Is(err, common.ErrSessionRevoked) {
		t.Fatal("unknown availability accepted", err)
	}
}

func TestMachineRoleCannotEstablishHumanSession(t *testing.T) {
	f := newRefreshFixture()
	f.roles.role.Code = "platform_service"
	f.admins.account.Password, _ = identity.HashPassword("fixture-password")
	if _, err := NewAdminLoginLogic(context.Background(), f.sc).AdminLogin(&types.AdminLoginReq{Account: "fixture-admin", Password: "fixture-password"}); !errors.Is(err, common.ErrRoleForbidden) {
		t.Fatal("machine role obtained a human login", err)
	}
	if _, err := runFixtureRefresh(f, refreshFixtureToken(t, "admin")); !errors.Is(err, common.ErrSessionRevoked) {
		t.Fatal("machine role refreshed a human session", err)
	}
	c := common.CustomClaims{UserId: 7, UserType: "admin", Roles: []string{"platform_service"}, ClientID: "platform-admin"}
	if _, err := CurrentAdminAccess(context.Background(), f.sc, &c); !errors.Is(err, common.ErrSessionRevoked) {
		t.Fatal("machine role passed human introspection", err)
	}
}
