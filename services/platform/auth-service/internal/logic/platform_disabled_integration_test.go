package logic

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/askxuan/auth-service/internal/model"
	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/auth-service/internal/types"
	"github.com/askxuan/common"
	"github.com/askxuan/common/identity"
	"github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Real SQL row locks and Redis challenges are required: mocks cannot establish
// whether an owner activation can race past a platform disable.
func TestPlatformDisablePriorityMySQL(t *testing.T) {
	dsn, redisAddr := os.Getenv("ACCOUNT_SYSTEM_TEST_DSN"), os.Getenv("ACCOUNT_SYSTEM_TEST_REDIS")
	if dsn == "" || redisAddr == "" {
		t.Skip("requires isolated account-system MySQL and Redis")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil || os.Getenv("ACCOUNT_SYSTEM_TEST_ISOLATED") != "1" || cfg.DBName != "askxuan_account_test" || cfg.Net != "tcp" || !strings.HasPrefix(cfg.Addr, "127.0.0.1:") || !strings.HasPrefix(redisAddr, "127.0.0.1:") {
		t.Fatal("refusing non-fixture databases")
	}
	cfg.DBName = "askxuan_auth"
	db := sqlx.NewMysql(cfg.FormatDSN())
	sqlx.DisableLog()
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	var count int64
	if err = db.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM information_schema.schemata WHERE schema_name='askxuan_account_test'"); err != nil || count != 1 {
		t.Fatal("fixture marker missing", err)
	}
	if err = db.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM admin_account WHERE id BETWEEN 970001 AND 970099"); err != nil || count != 0 {
		t.Fatal("fixture IDs occupied", err)
	}
	r := redis.MustNewRedis(redis.RedisConf{Host: redisAddr, Type: "node"})
	challenges := &identity.Challenges{Redis: r, Secret: "isolated-platform-disable-fixture"}
	accounts := &identity.Accounts{DB: db, Challenges: challenges}
	ownedRoles := []int64{}
	createdSessions := []string{}
	role := func(code string, id int64) int64 {
		var found int64
		if e := db.QueryRowCtx(ctx, &found, "SELECT id FROM role WHERE code=?", code); e == nil {
			return found
		} else if !errors.Is(e, sqlx.ErrNotFound) {
			t.Fatal(e)
		}
		if _, e := db.ExecCtx(ctx, "INSERT INTO role(id,name,code) VALUES(?,?,?)", id, "platform-priority-fixture", code); e != nil {
			t.Fatal(e)
		}
		ownedRoles = append(ownedRoles, id)
		return id
	}
	role("platform_super", 970091)
	masterID := role("master", 970092)
	cleanup := func() {
		for _, sid := range createdSessions {
			if e := identity.RevokeSession(context.Background(), r, sid); e != nil {
				t.Error(e)
			}
		}
		for _, q := range []string{
			"DELETE FROM auth_identity WHERE domain='admin' AND user_id BETWEEN 970001 AND 970099",
			"DELETE FROM master_invitation WHERE account_id BETWEEN 970001 AND 970099",
			"DELETE FROM admin_account WHERE id BETWEEN 970001 AND 970099 AND account LIKE 'priority_fixture_%'",
			"DELETE FROM askxuan_temple.temple_admin WHERE account_id BETWEEN 970001 AND 970099",
			"DELETE FROM askxuan_master.master WHERE code IN ('M970002','M970003') AND dharma_name='priority-fixture'",
			"DELETE FROM askxuan_temple.temple WHERE code='T970001' AND name='priority-fixture'",
		} {
			if _, e := db.ExecCtx(context.Background(), q); e != nil {
				t.Error(e)
			}
		}
		for _, id := range ownedRoles {
			if _, e := db.ExecCtx(context.Background(), "DELETE FROM role WHERE id=? AND name='platform-priority-fixture'", id); e != nil {
				t.Error(e)
			}
		}
		for _, id := range []int64{970002, 970003} {
			_, _ = r.DelCtx(context.Background(), identity.EpochKey("admin", id))
		}
	}
	t.Cleanup(cleanup)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := db.ExecCtx(ctx, q, args...); e != nil {
			t.Fatal(e)
		}
	}
	exec("INSERT INTO askxuan_temple.temple(id,code,name,status) VALUES(970001,'T970001','priority-fixture','正常')")
	exec("INSERT INTO askxuan_master.master(id,code,dharma_name,temple_code,auth_status,platform_status,shelf_status,manage_by) VALUES(970002,'M970002','priority-fixture','T970001','已认证','normal','off_shelf','temple'),(970003,'M970003','priority-fixture','T970001','已认证','normal','off_shelf','temple')")
	hash, e := identity.HashPassword("IsolatedFixture#970002")
	if e != nil {
		t.Fatal(e)
	}
	exec("INSERT INTO admin_account(id,account,password,role_id,temple_id,master_id,status) VALUES(970002,'priority_fixture_pending',?,?,'T970001','M970002','disabled'),(970003,'priority_fixture_revoked',?,?,'T970001','M970003','disabled')", hash, masterID, hash, masterID)
	exec("INSERT INTO master_invitation(id,account_id,temple_code,master_code,email,username,status) VALUES(970002,970002,'T970001','M970002','priority-pending@example.com','priority_fixture_pending','pending'),(970003,970003,'T970001','M970003','priority-revoked@example.com','priority_fixture_revoked','revoked')")
	exec("INSERT INTO auth_identity(domain,user_id,username,email,password_hash,agreement_version,verified_at) VALUES('admin',970003,'priority_fixture_revoked','priority-revoked@example.com',?,?,NOW())", hash, identity.AgreementVersion)
	sc := &svc.ServiceContext{DB: db, AdminAccountModel: model.NewAdminAccountModel(db), RoleModel: model.NewRoleModel(db), Accounts: accounts, SessionRedis: r, Redis: r}
	sc.Config.Auth.AccessSecret = refreshTestSecret
	sc.Config.Auth.AccessExpire = 60
	sc.Config.Auth.RefreshExpire = 600
	toggle := func(id int64, status string) error {
		_, e := NewAdminAccountStatusLogic(ctx, sc).AdminAccountStatus(&types.AdminAccountStatusReq{Id: id, Status: status})
		return e
	}
	assertState := func(id int64, status string, flag bool, version int64) {
		t.Helper()
		a, e := sc.AdminAccountModel.FindByID(ctx, id)
		if e != nil {
			t.Fatal("cannot read fixture state", e)
		}
		if a.Status != status || a.PlatformDisabled != flag || (version >= 0 && a.AuthzVersion != version) {
			t.Fatalf("state %d: status=%v flag=%v version=%v err=%v", id, a.Status, a.PlatformDisabled, a.AuthzVersion, e)
		}
	}
	activate := func() error {
		if e := challenges.Save(ctx, "email", "admin:activate:priority-pending@example.com", "912345", 60); e != nil {
			return e
		}
		return accounts.ActivateManagedMaster(ctx, "priority-pending@example.com", "IsolatedFixture#970002", "912345", identity.AgreementVersion)
	}
	t.Run("platform release preserves pending activation", func(t *testing.T) {
		if e := toggle(970002, "disabled"); e != nil {
			t.Fatal(e)
		}
		assertState(970002, "disabled", true, 1)
		detail, e := AdminAccountDetail(ctx, sc, 970002)
		if e != nil || !detail.Account.PlatformDisabled || detail.LoginAllowed || !strings.Contains(strings.Join(detail.BlockedReasons, ";"), "平台已停用") {
			t.Fatal("detail did not expose independent platform block", e)
		}
		listed, e := NewAdminAccountListLogic(ctx, sc).AdminAccountList(&types.AdminAccountListReq{Keyword: "priority_fixture_pending", Page: 1, Size: 20})
		if e != nil || len(listed.List) != 1 || !listed.List[0].PlatformDisabled {
			t.Fatal("list did not expose independent platform block", e)
		}
		managed, e := accounts.ManagedInvitations(ctx, "T970001")
		if e != nil {
			t.Fatal(e)
		}
		found := false
		for _, row := range managed {
			if row.AccountID == 970002 {
				found = row.PlatformDisabled && row.AccountStatus == "disabled" && row.ActivationStatus == "pending"
			}
		}
		if !found {
			t.Fatal("managed list conflated platform state and activation")
		}
		if e := toggle(970002, "disabled"); e != nil {
			t.Fatal(e)
		}
		assertState(970002, "disabled", true, 1)
		if e := activate(); e == nil {
			t.Fatal("platform block bypassed by owner activation")
		}
		if e := toggle(970002, "enabled"); e != nil {
			t.Fatal(e)
		}
		assertState(970002, "disabled", false, 2)
		if e := toggle(970002, "enabled"); e == nil {
			t.Fatal("pending owner proof bypassed by second platform release")
		}
		if e := activate(); e != nil {
			t.Fatal(e)
		}
		assertState(970002, "enabled", false, 3)
	})
	t.Run("platform release preserves organisation revoke", func(t *testing.T) {
		if e := toggle(970003, "disabled"); e != nil {
			t.Fatal(e)
		}
		if e := accounts.RestoreManagedMaster(ctx, "T970001", 970003); e == nil {
			t.Fatal("organisation restore bypassed platform block")
		}
		if e := toggle(970003, "enabled"); e != nil {
			t.Fatal(e)
		}
		assertState(970003, "disabled", false, 2)
		if e := accounts.RestoreManagedMaster(ctx, "T970001", 970003); e != nil {
			t.Fatal(e)
		}
		assertState(970003, "enabled", false, 3)
	})
	t.Run("simultaneous activation and disable always end blocked", func(t *testing.T) {
		for i := 0; i < 4; i++ {
			exec("DELETE FROM auth_identity WHERE domain='admin' AND user_id=970002")
			exec("UPDATE master_invitation SET status='pending' WHERE id=970002")
			exec("UPDATE admin_account SET status='disabled',platform_disabled=0 WHERE id=970002")
			gate := make(chan struct{})
			var wg sync.WaitGroup
			var disableErr error
			wg.Add(2)
			go func() { defer wg.Done(); <-gate; _ = activate() }()
			go func() { defer wg.Done(); <-gate; disableErr = toggle(970002, "disabled") }()
			close(gate)
			wg.Wait()
			if disableErr != nil {
				t.Fatal(disableErr)
			}
			assertState(970002, "disabled", true, -1)
		}
	})
	t.Run("simultaneous organisation restore and disable always end blocked", func(t *testing.T) {
		for i := 0; i < 4; i++ {
			exec("UPDATE master_invitation SET status='revoked' WHERE id=970003")
			exec("UPDATE admin_account SET status='disabled',platform_disabled=0 WHERE id=970003")
			gate := make(chan struct{})
			var wg sync.WaitGroup
			var disableErr error
			wg.Add(2)
			go func() { defer wg.Done(); <-gate; _ = accounts.RestoreManagedMaster(ctx, "T970001", 970003) }()
			go func() { defer wg.Done(); <-gate; disableErr = toggle(970003, "disabled") }()
			close(gate)
			wg.Wait()
			if disableErr != nil {
				t.Fatal(disableErr)
			}
			assertState(970003, "disabled", true, -1)
		}
	})
	t.Run("stale profile editor cannot change status or platform switch", func(t *testing.T) {
		a, e := sc.AdminAccountModel.FindByID(ctx, 970003)
		if e != nil {
			t.Fatal(e)
		}
		a.Status = "enabled"
		a.PlatformDisabled = false
		a.Name = "profile updated"
		if e = updateAdminAccountWithBinding(ctx, db, a, false, false); e != nil {
			t.Fatal(e)
		}
		assertState(970003, "disabled", true, -1)
	})
	t.Run("legacy inconsistent enabled row still denies human access and refresh", func(t *testing.T) {
		exec("UPDATE admin_account SET status='enabled',platform_disabled=1 WHERE id=970003")
		a, e := sc.AdminAccountModel.FindByID(ctx, 970003)
		if e != nil {
			t.Fatal(e)
		}
		info := common.TokenInfo{UserId: a.Id, UserType: "admin", Roles: []string{"master"}, RoleID: masterID, AuthzScope: "master", AccountVersion: a.AuthzVersion}
		at, _ := common.GenAccessToken(refreshTestSecret, info, 60)
		claims, _ := common.ParseToken(refreshTestSecret, at)
		if _, e = CurrentAdminAccess(ctx, sc, claims); !errors.Is(e, common.ErrSessionRevoked) {
			t.Fatal("access error must terminate", e)
		}
		claims.UserType = "master"
		if _, e = NewRefreshLogic(ctx, sc).refreshIdentity(claims); !errors.Is(e, common.ErrSessionRevoked) {
			t.Fatal("refresh error must terminate", e)
		}
		if _, e = NewAdminLoginLogic(ctx, sc).AdminLogin(&types.AdminLoginReq{Account: a.Account, Password: "IsolatedFixture#970002"}); !errors.Is(e, common.ErrUserDisabled) {
			t.Fatal("platform blocked fresh login", e)
		}
	})
	t.Run("old work credentials never revive after platform release", func(t *testing.T) {
		exec("UPDATE master_invitation SET status='activated' WHERE id=970003")
		exec("UPDATE admin_account SET status='enabled',platform_disabled=0 WHERE id=970003")
		login, e := NewAdminLoginLogic(ctx, sc).AdminLogin(&types.AdminLoginReq{Account: "priority_fixture_revoked", Password: "IsolatedFixture#970002"})
		if e != nil {
			t.Fatal(e)
		}
		access, _ := common.ParseToken(refreshTestSecret, login.AccessToken)
		createdSessions = append(createdSessions, access.SessionID)
		for _, status := range []string{"disabled", "enabled"} {
			if e = toggle(970003, status); e != nil {
				t.Fatal(e)
			}
			if _, e = CurrentAdminAccess(ctx, sc, access); !errors.Is(e, common.ErrSessionRevoked) {
				t.Fatalf("old access survived %s: %v", status, e)
			}
			if _, e = NewRefreshLogic(ctx, sc).Refresh(&types.RefreshReq{RefreshToken: login.RefreshToken}); !errors.Is(e, common.ErrSessionRevoked) {
				t.Fatalf("old refresh survived %s: %v", status, e)
			}
		}
	})
}
