package logic

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/askxuan/auth-service/internal/model"
	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/auth-service/internal/types"
	"github.com/askxuan/common"
	"github.com/askxuan/common/identity"
	"github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestAccountDetailsMySQL(t *testing.T) {
	dsn := os.Getenv("ACCOUNT_SYSTEM_TEST_DSN")
	if dsn == "" || os.Getenv("ACCOUNT_SYSTEM_TEST_ISOLATED") != "1" {
		t.Skip("requires dedicated disposable MySQL")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil || cfg.DBName != "askxuan_account_test" {
		t.Fatal("refusing non-test database")
	}
	sqlx.DisableLog()
	ctx := context.Background()
	fixtureDB := sqlx.NewMysql(dsn)
	var dbName string
	if fixtureDB.QueryRowCtx(ctx, &dbName, "SELECT DATABASE()") != nil || dbName != cfg.DBName {
		t.Fatal("isolated database unavailable")
	}
	ddl, err := os.ReadFile("../../../../../scripts/dev/account-system/fixture.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range strings.Split(string(ddl), ";") {
		if strings.TrimSpace(q) != "" {
			if _, err = fixtureDB.ExecCtx(ctx, q); err != nil {
				t.Fatal(err)
			}
		}
	}
	db := sqlx.NewMysql(strings.Replace(dsn, "/askxuan_account_test", "/askxuan_auth", 1))
	s := &svc.ServiceContext{DB: db, AdminAccountModel: model.NewAdminAccountModel(db), RoleModel: model.NewRoleModel(db)}
	var count int64
	if db.QueryRowCtx(ctx, &count, `SELECT COUNT(*) FROM admin_account WHERE id BETWEEN 930001 AND 930099`) != nil || count != 0 {
		t.Fatal("fixture accounts already present; refusing overwrite")
	}
	t.Cleanup(func() {
		for _, q := range []string{`DELETE FROM master_invitation WHERE account_id BETWEEN 930001 AND 930099`, `DELETE FROM auth_identity WHERE user_id BETWEEN 930001 AND 930099 AND domain='admin'`, `DELETE FROM admin_account WHERE id BETWEEN 930001 AND 930099`, `DELETE FROM role WHERE id BETWEEN 930001 AND 930099`, `DELETE FROM askxuan_temple.temple WHERE code='T930001'`, `DELETE FROM askxuan_master.master WHERE code='M930001'`} {
			if _, err := db.ExecCtx(context.Background(), q); err != nil {
				t.Error(err)
			}
		}
	})
	seed := []string{
		`INSERT IGNORE INTO role(id,name,code,authz_scope) VALUES(930001,'Fixture master','master','master'),(930002,'Fixture temple','temple_admin','temple'),(930003,'Fixture platform','fixture_operator_930001','platform'),(930004,'Fixture super','platform_super','platform')`,
		`INSERT INTO admin_account(id,account,password,name,role_id,temple_id,master_id,status) VALUES(930001,'fixture_activated','secret-hash-never-exported','Activated',930001,'T930001','M930001','enabled'),(930002,'fixture_pending','','Pending',930001,'T930001','M930001','disabled'),(930003,'fixture_legacy','legacy-hash-never-exported','Legacy',930003,'','','enabled')`,
		`INSERT INTO auth_identity(domain,user_id,username,email,password_hash,verified_at) VALUES('admin',930001,'fixture_activated','verified@example.test','identity-hash-never-exported',NOW())`,
		`INSERT INTO master_invitation VALUES(930001,930001,'T930001','M930001','verified@example.test','fixture_activated','activated'),(930002,930002,'T930001','M930001','pending@example.test','fixture_pending','pending')`,
		`INSERT INTO askxuan_temple.temple(id,code,name,status,auth_status,platform_status,shelf_status) VALUES(930001,'T930001','Fixture temple','正常','certified','normal','off_shelf')`,
		`INSERT INTO askxuan_master.master(id,code,dharma_name,temple_code,auth_status,platform_status,shelf_status) VALUES(930001,'M930001','Fixture master','T930001','已认证','normal','off_shelf')`,
	}
	var masterRoleID int64
	for i, q := range seed {
		if i == 1 {
			if err := db.QueryRowCtx(ctx, &masterRoleID, `SELECT id FROM role WHERE code='master'`); err != nil {
				t.Fatal(err)
			}
			q = strings.ReplaceAll(q, ",930001,'T930001'", ","+strconv.FormatInt(masterRoleID, 10)+",'T930001'")
		}
		if _, err := db.ExecCtx(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	t.Run("verified identity and off shelf bindings allow fulfillment login", func(t *testing.T) {
		out, err := AdminAccountDetail(ctx, s, 930001)
		if err != nil {
			t.Fatal(err)
		}
		if !out.LoginAllowed || !out.Identity.EmailVerified || !out.Identity.CanResetPassword || out.Identity.ActivationStatus != "activated" || len(out.Bindings) != 2 || out.Bindings[0].ShelfStatus != "off_shelf" || out.Bindings[1].ShelfStatus != "off_shelf" {
			t.Fatalf("wrong detail %+v", out)
		}
		b, _ := json.Marshal(out)
		for _, secret := range []string{"secret-hash", "identity-hash", "password", "password_hash"} {
			if strings.Contains(string(b), secret) {
				t.Fatal("credential material leaked")
			}
		}
	})
	t.Run("pending invitation is not a verified identity", func(t *testing.T) {
		out, err := AdminAccountDetail(ctx, s, 930002)
		if err != nil || out.LoginAllowed || out.Identity.CredentialState != "pending_activation" || out.Identity.EmailVerified || out.Identity.CanResetPassword || out.Identity.InvitationEmail != "pending@example.test" {
			t.Fatalf("pending state incorrect %+v %v", out, err)
		}
	})
	t.Run("legacy work passwords remain valid without invented email", func(t *testing.T) {
		out, err := AdminAccountDetail(ctx, s, 930003)
		if err != nil || !out.LoginAllowed || out.Identity.CredentialState != "legacy_password" || out.Identity.Email != "" || out.Identity.CanResetPassword {
			t.Fatalf("legacy state incorrect %+v %v", out, err)
		}
	})
	t.Run("platform ban differs from display shelf", func(t *testing.T) {
		if _, err := db.ExecCtx(ctx, `UPDATE askxuan_temple.temple SET platform_status='banned' WHERE code='T930001'`); err != nil {
			t.Fatal(err)
		}
		out, err := AdminAccountDetail(ctx, s, 930001)
		if err != nil || out.LoginAllowed || len(out.BlockedReasons) == 0 {
			t.Fatalf("banned temple allowed login: %+v %v", out, err)
		}
		if _, err := db.ExecCtx(ctx, `UPDATE askxuan_temple.temple SET platform_status='normal' WHERE code='T930001'`); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("reverse subject and role filters use exact bindings", func(t *testing.T) {
		out, err := NewAdminAccountListLogic(ctx, s).AdminAccountList(&types.AdminAccountListReq{TempleId: "T930001", MasterId: "M930001", RoleId: masterRoleID, Page: 1, Size: 20})
		if err != nil || out.Total != 2 || len(out.List) != 2 {
			t.Fatalf("subject filter incorrect %+v %v", out, err)
		}
		no, err := NewAdminAccountListLogic(ctx, s).AdminAccountList(&types.AdminAccountListReq{TempleId: "T930001' OR 1=1 --", Page: 1, Size: 20})
		if err != nil || no.Total != 0 {
			t.Fatal("subject query was not parameterized")
		}
		if _, err = NewAdminAccountListLogic(ctx, s).AdminAccountList(&types.AdminAccountListReq{RoleId: -1}); err == nil {
			t.Fatal("negative role filter accepted")
		}
	})
	t.Run("managed account activation and login switch stay distinct", func(t *testing.T) {
		if _, err := db.ExecCtx(ctx, `UPDATE admin_account SET status='disabled' WHERE id=930001`); err != nil {
			t.Fatal(err)
		}
		rows, err := (&identity.Accounts{DB: db}).ManagedInvitations(ctx, "T930001")
		if err != nil {
			t.Fatal(err)
		}
		var found bool
		for _, row := range rows {
			if row.AccountID == 930001 {
				found = true
				if row.ActivationStatus != "activated" || row.Status != "activated" || row.AccountStatus != "disabled" {
					t.Fatalf("activation masked platform disable: %+v", row)
				}
			}
		}
		if !found {
			t.Fatal("managed account missing")
		}
		if _, err = NewAdminAccountStatusLogic(ctx, s).AdminAccountStatus(&types.AdminAccountStatusReq{Id: 930002, Status: "enabled"}); err == nil {
			t.Fatal("pending activation bypassed by platform toggle")
		}
		if _, err := db.ExecCtx(ctx, `UPDATE admin_account SET status='enabled' WHERE id=930001`); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("ordinary account edits preserve the live login switch", func(t *testing.T) {
		stale := &model.AdminAccount{Id: 930003, Name: "Legacy edited", RoleId: 930003, Status: "enabled"}
		if _, err := db.ExecCtx(ctx, `UPDATE admin_account SET status='disabled' WHERE id=930003`); err != nil {
			t.Fatal(err)
		}
		if err := updateAdminAccountWithBinding(ctx, db, stale, false, false); err != nil {
			t.Fatal(err)
		}
		saved, err := s.AdminAccountModel.FindByID(ctx, 930003)
		if err != nil || saved.Status != "disabled" || stale.Status != "disabled" {
			t.Fatal("stale edit re-enabled disabled account", err)
		}
		stale.Status = "disabled"
		if _, err := db.ExecCtx(ctx, `UPDATE admin_account SET status='enabled' WHERE id=930003`); err != nil {
			t.Fatal(err)
		}
		if err := updateAdminAccountWithBinding(ctx, db, stale, false, false); err != nil {
			t.Fatal(err)
		}
		saved, err = s.AdminAccountModel.FindByID(ctx, 930003)
		if err != nil || saved.Status != "enabled" {
			t.Fatal("stale edit overwrote newer enabled switch", err)
		}
	})
	t.Run("transaction guard cannot bypass a revoked invitation", func(t *testing.T) {
		if _, err := db.ExecCtx(ctx, `UPDATE master_invitation SET status='revoked' WHERE account_id=930002`); err != nil {
			t.Fatal(err)
		}
		if err := updateAccountStatus(ctx, s, &model.AdminAccount{Id: 930002, RoleId: masterRoleID}, "enabled"); err == nil {
			t.Fatal("transaction bypassed revoked assignment")
		}
		saved, err := s.AdminAccountModel.FindByID(ctx, 930002)
		if err != nil || saved.Status != "disabled" {
			t.Fatal("revoked work account re-enabled", err)
		}
	})
	t.Run("session revocation changes only version and rejects old snapshot", func(t *testing.T) {
		before, err := s.AdminAccountModel.FindByID(ctx, 930001)
		if err != nil {
			t.Fatal(err)
		}
		role, err := s.RoleModel.FindByID(ctx, before.RoleId)
		if err != nil {
			t.Fatal(err)
		}
		c := &common.CustomClaims{UserId: before.Id, UserType: "admin", Roles: []string{role.Code}, RoleID: role.Id, AccountVersion: before.AuthzVersion, RoleVersion: role.AuthzVersion, AuthzScope: effectiveRoleScope(role)}
		if !authorizationVersionMatches(c, before, role) {
			t.Fatal("fixture original version mismatch")
		}
		if _, err = RevokeAdminAccountSessions(ctx, s, 930001); err != nil {
			t.Fatal(err)
		}
		after, err := s.AdminAccountModel.FindByID(ctx, 930001)
		if err != nil || after.AuthzVersion != before.AuthzVersion+1 || after.Password != before.Password || after.Status != before.Status || after.MasterId != before.MasterId || authorizationVersionMatches(c, after, role) {
			t.Fatalf("revoke changed state/accepted old snapshot: %+v %v", after, err)
		}
	})
}
