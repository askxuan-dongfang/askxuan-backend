package logic

import (
	"context"
	"github.com/askxuan/auth-service/internal/model"
	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/askxuan/common/adminpolicy"
	"github.com/askxuan/common/identity"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"strings"
	"testing"
)

func TestTempleTeamIsolationAndRevocationIntegration(t *testing.T) {
	dsn := os.Getenv("TEMPLE_OPS_AUTH_DSN")
	if dsn == "" {
		t.Skip("requires task-only MySQL")
	}
	if !strings.Contains(dsn, "@tcp(127.0.0.1:63336)/askxuan_auth?") {
		t.Fatal("refuse non-isolated database")
	}
	sqlx.DisableLog()
	db := sqlx.NewMysql(dsn)
	ctx := context.Background()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := db.ExecCtx(ctx, q, args...); e != nil {
			t.Fatal(e)
		}
	}
	exec(`INSERT IGNORE INTO askxuan_temple.temple(code,name,region,type,sect,status) VALUES('T_OPS','岗位测试','测试地区','汉传佛教','禅宗','正常')`)
	exec(`INSERT IGNORE INTO admin_account(account,password,name,role_id,temple_id,master_id,shop_id,status) SELECT 'templeops-owner','unused','岗位测试管理员',id,'T_OPS','',0,'enabled' FROM role WHERE code='temple_admin'`)
	var owner model.AdminAccount
	accountModel := model.NewAdminAccountModel(db)
	loaded, e := accountModel.FindByAccount(ctx, "templeops-owner")
	if e != nil {
		t.Fatal(e)
	}
	owner = *loaded
	s := &svc.ServiceContext{DB: db, AdminAccountModel: accountModel, RoleModel: model.NewRoleModel(db)}
	role, e := s.RoleModel.FindByID(ctx, owner.RoleId)
	if e != nil {
		t.Fatal(e)
	}
	c := &common.CustomClaims{UserId: owner.Id, UserType: "admin", ClientID: "temple-admin", Roles: []string{"temple_admin"}, TempleCode: "T_OPS", RoleID: owner.RoleId, AccountVersion: owner.AuthzVersion, RoleVersion: role.AuthzVersion, AuthzScope: "temple"}
	t.Cleanup(func() {
		exec(`DELETE i FROM auth_identity i JOIN admin_account a ON i.domain='admin' AND i.user_id=a.id WHERE a.account LIKE 'templeops-%'`)
		exec(`DELETE ta FROM askxuan_temple.temple_admin ta JOIN admin_account a ON a.id=ta.account_id WHERE a.account LIKE 'templeops-%'`)
		exec(`DELETE FROM admin_account WHERE account LIKE 'templeops-%'`)
		exec(`DELETE FROM askxuan_temple.temple WHERE code='T_OPS'`)
	})
	id, e := SaveTempleTeam(ctx, s, c, TempleTeamInput{Account: "templeops-reception", Password: "TemporaryPassword128!", Name: "测试接待", Role: "temple_reception", Status: "enabled"})
	if e != nil {
		t.Fatal(e)
	}
	a, e := accountModel.FindByID(ctx, id)
	if e != nil {
		t.Fatal(e)
	}
	r, e := s.RoleModel.FindByID(ctx, a.RoleId)
	if e != nil {
		t.Fatal(e)
	}
	staff := &common.CustomClaims{UserId: id, UserType: "admin", ClientID: "temple-admin", Roles: []string{r.Code}, RoleID: r.Id, RoleVersion: r.AuthzVersion, AccountVersion: a.AuthzVersion, AuthzScope: "temple", TempleCode: "T_OPS", Permissions: adminpolicy.TemplePermissions(r.Code)}
	if _, e := CurrentAdminAccess(ctx, s, staff); e != nil {
		t.Fatal("staff login/access identity", e)
	}
	if _, e := SaveTempleTeam(ctx, s, staff, TempleTeamInput{}); e == nil {
		t.Fatal("staff managed accounts")
	}
	other := *c
	other.TempleCode = "OTHER"
	if _, e := SaveTempleTeam(ctx, s, &other, TempleTeamInput{ID: id, Revision: a.AuthzVersion, Name: "越权", Role: "temple_finance", Status: "enabled"}); e == nil {
		t.Fatal("cross-temple write")
	}
	if _, e := SaveTempleTeam(ctx, s, c, TempleTeamInput{ID: id, Revision: a.AuthzVersion, Name: "测试财务", Role: "temple_finance", Status: "enabled"}); e != nil {
		t.Fatal(e)
	}
	if _, e := CurrentAdminAccess(ctx, s, staff); e == nil {
		t.Fatal("old access survives role change")
	}
	if _, e := SaveTempleTeam(ctx, s, c, TempleTeamInput{ID: id, Revision: a.AuthzVersion, Name: "旧版本", Role: "temple_reception", Status: "enabled"}); e == nil {
		t.Fatal("stale revision overwrote role")
	}
	rows, e := TempleTeamList(ctx, s, c)
	if e != nil || len(rows) != 1 || rows[0].Role != "temple_finance" {
		t.Fatal("platform/shared role row mismatch", e, rows)
	}
	// Staff promoted from a previously verified identity must not keep the old login credential.
	exec(`INSERT INTO auth_identity(domain,user_id,username,email,password_hash,verified_at) VALUES('admin',?,'templeops-verified','templeops-verified@example.invalid','old-hash',NOW())`, id)
	if _, e := SaveTempleTeam(ctx, s, c, TempleTeamInput{ID: id, Revision: rows[0].Revision, Name: "测试财务", Role: "temple_finance", Status: "disabled", Password: "ReplacementPassword128!"}); e != nil {
		t.Fatal(e)
	}
	var identityHash string
	if e := db.QueryRowCtx(ctx, &identityHash, `SELECT password_hash FROM auth_identity WHERE domain='admin' AND user_id=?`, id); e != nil || !identity.CheckPassword(identityHash, "ReplacementPassword128!") {
		t.Fatal("verified identity credential was not reset", e)
	}
	if _, e := CurrentAdminAccess(ctx, s, staff); e == nil {
		t.Fatal("disabled member retained access")
	}
}
