package identity

import (
	"context"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"sync"
	"sync/atomic"
	"testing"
)

func TestDualTrackOnboardingIntegration(t *testing.T) {
	dsn, host := os.Getenv("IDENTITY_TEST_MYSQL"), os.Getenv("IDENTITY_TEST_REDIS")
	if dsn == "" || host == "" {
		t.Skip("requires isolated onboarding-schema.sql")
	}
	ctx := context.Background()
	db := sqlx.NewMysql(dsn)
	r := redis.MustNewRedis(redis.RedisConf{Host: host, Type: "node"})
	mail := &testMailer{}
	s := Accounts{DB: db, Challenges: &Challenges{Redis: r, Secret: RandomID()}, Mailer: mail}
	prefix := "onboard_" + RandomID()[:12]
	password := "Onboarding-test-password-2026!"
	ids := []int64{}
	entities := []string{}
	defer func() {
		for _, id := range ids {
			db.ExecCtx(ctx, "DELETE e FROM onboarding_event e JOIN onboarding_application a ON a.id=e.application_id WHERE a.account_id=?", id)
			db.ExecCtx(ctx, "DELETE FROM onboarding_evidence WHERE account_id=?", id)
			db.ExecCtx(ctx, "DELETE FROM onboarding_application WHERE account_id=?", id)
			db.ExecCtx(ctx, "DELETE FROM master_invitation WHERE account_id=?", id)
			db.ExecCtx(ctx, "DELETE FROM auth_identity WHERE domain='admin' AND user_id=?", id)
			db.ExecCtx(ctx, "DELETE FROM askxuan_temple.temple_admin WHERE account_id=?", id)
			db.ExecCtx(ctx, "DELETE FROM admin_account WHERE id=?", id)
		}
		for _, code := range entities {
			db.ExecCtx(ctx, "DELETE FROM askxuan_master.master_profile_ext WHERE master_code=?", code)
			db.ExecCtx(ctx, "DELETE FROM askxuan_master.master WHERE code=?", code)
			db.ExecCtx(ctx, "DELETE FROM askxuan_temple.temple WHERE code=?", code)
		}
	}()
	create := func(kind, suffix string) int64 {
		t.Helper()
		name := prefix + suffix
		email := name + "@example.test"
		if e := s.SendCode(ctx, "admin", email, kind+"_register"); e != nil {
			t.Fatal(e)
		}
		id, e := s.RegisterWork(ctx, kind, email, name, password, mail.code, AgreementVersion)
		if e != nil {
			t.Fatal(e)
		}
		ids = append(ids, id)
		if _, e = s.PasswordLogin(ctx, "admin", email, password); e != nil {
			t.Fatal(e)
		}
		return id
	}
	master := create("master", "m")
	temple := create("temple", "t")
	if _, e := s.RegisterWork(ctx, "temple_managed", "bad@example.test", "baduser", password, "000000", AgreementVersion); e == nil {
		t.Fatal("self provisioned managed identity")
	}
	profile := func(id int64, kind string) WorkProfile {
		t.Helper()
		evidence := RandomID()
		if _, e := db.ExecCtx(ctx, "INSERT INTO onboarding_evidence(id,account_id,filename,media_type,content) VALUES (?,?,'fixture.pdf','application/pdf',?)", evidence, id, []byte("%PDF-fixture")); e != nil {
			t.Fatal(e)
		}
		return WorkProfile{Name: "测试名称", LegalName: "测试本人", Contact: "测试联系方式", Region: "测试地区", Address: "测试地址", RegistrationNumber: "TEST-REGISTRATION", Belief: "han_buddhism", Sect: "测试宗派", Position: "测试职务", EvidenceIDs: []string{evidence}}
	}
	mprofile := profile(master, "master")
	tprofile := profile(temple, "temple")
	app, e := s.WorkApplication(ctx, master)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.SaveWorkApplication(ctx, master, tprofile, app.Revision, true); e == nil {
		t.Fatal("cross-account evidence accepted")
	}
	if _, e = s.SaveWorkApplication(ctx, master, WorkProfile{}, app.Revision, true); e == nil {
		t.Fatal("empty application submitted")
	}
	app, e = s.SaveWorkApplication(ctx, master, mprofile, app.Revision, true)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.SaveWorkApplication(ctx, master, mprofile, app.Revision, false); e == nil {
		t.Fatal("submitted version mutable")
	}
	if e = s.ReviewWorkApplication(ctx, 999999, app.ID, app.Revision, false, ""); e == nil {
		t.Fatal("rejection lacks reason")
	}
	if e = s.ReviewWorkApplication(ctx, 999999, app.ID, app.Revision, false, "请补充资料"); e != nil {
		t.Fatal(e)
	}
	app, e = s.WorkApplication(ctx, master)
	if e != nil {
		t.Fatal(e)
	}
	if app.Status != "rejected" || app.ReviewNote != "请补充资料" {
		t.Fatal("missing review result")
	}
	app, e = s.SaveWorkApplication(ctx, master, mprofile, app.Revision, true)
	if e != nil {
		t.Fatal(e)
	}
	a, e := s.Find(ctx, "admin", prefix+"m")
	if e != nil {
		t.Fatal(e)
	}
	sid, e := s.IssueSession(ctx, "admin", master, a.PasswordHash, 120)
	if e != nil {
		t.Fatal(e)
	}
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if s.ReviewWorkApplication(ctx, 999999, app.ID, app.Revision, true, "资料核验通过") == nil {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()
	if accepted.Load() != 1 {
		t.Fatalf("concurrent approvals=%d", accepted.Load())
	}
	app, e = s.WorkApplication(ctx, master)
	if e != nil {
		t.Fatal(e)
	}
	entities = append(entities, app.EntityCode)
	if CheckSession(ctx, r, sid, "admin", master) == nil {
		t.Fatal("approval retained applicant session")
	}
	var managed string
	if e = db.QueryRowCtx(ctx, &managed, "SELECT manage_by FROM askxuan_master.master WHERE code=?", app.EntityCode); e != nil || managed != "platform" {
		t.Fatal("independent master binding", e)
	}
	ta, e := s.WorkApplication(ctx, temple)
	if e != nil {
		t.Fatal(e)
	}
	ta, e = s.SaveWorkApplication(ctx, temple, tprofile, ta.Revision, true)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.ReviewWorkApplication(ctx, 999999, ta.ID, ta.Revision, true, ""); e != nil {
		t.Fatal(e)
	}
	ta, e = s.WorkApplication(ctx, temple)
	if e != nil {
		t.Fatal(e)
	}
	entities = append(entities, ta.EntityCode)
	if e = s.InviteManagedMaster(ctx, temple, ta.EntityCode, app.EntityCode, prefix+"i@example.test", prefix+"i"); e == nil {
		t.Fatal("temple adopted independent master")
	}
	managedCode := "Q" + RandomID()[:12]
	entities = append(entities, managedCode)
	if _, e = db.ExecCtx(ctx, `INSERT INTO askxuan_master.master(code,dharma_name,temple_code,position,sect,type,auth_status,manage_by) VALUES (?,'纳管测试',?,'测试','测试','佛教','已认证','temple')`, managedCode, ta.EntityCode); e != nil {
		t.Fatal(e)
	}
	if e = s.InviteManagedMaster(ctx, temple, "other-temple", managedCode, prefix+"i@example.test", prefix+"i"); e == nil {
		t.Fatal("cross-temple invite")
	}
	if e = s.InviteManagedMaster(ctx, temple, ta.EntityCode, managedCode, prefix+"i@example.test", prefix+"i"); e != nil {
		t.Fatal(e)
	}
	invs, e := s.ManagedInvitations(ctx, ta.EntityCode)
	if e != nil || len(invs) != 1 {
		t.Fatal(e)
	}
	inv := invs[0]
	ids = append(ids, inv.AccountID)
	if e = s.SendCode(ctx, "admin", inv.Email, "activate"); e != nil {
		t.Fatal(e)
	}
	activation := mail.code
	if e = s.ActivateManagedMaster(ctx, inv.Email, password, activation, AgreementVersion); e != nil {
		t.Fatal(e)
	}
	if e = s.ActivateManagedMaster(ctx, inv.Email, password, activation, AgreementVersion); e == nil {
		t.Fatal("activation replay")
	}
	a, e = s.PasswordLogin(ctx, "admin", inv.Username, password)
	if e != nil {
		t.Fatal(e)
	}
	sid, e = s.IssueSession(ctx, "admin", a.UserID, a.PasswordHash, 120)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.RevokeManagedMaster(ctx, "other-temple", inv.ID); e == nil {
		t.Fatal("cross-temple revoke")
	}
	if e = s.RevokeManagedMaster(ctx, ta.EntityCode, inv.ID); e != nil {
		t.Fatal(e)
	}
	if CheckSession(ctx, r, sid, "admin", inv.AccountID) == nil {
		t.Fatal("revoked account session survived")
	}
	if e = s.RestoreManagedMaster(ctx, "other-temple", inv.ID); e == nil {
		t.Fatal("cross-temple restore")
	}
	if e = s.RestoreManagedMaster(ctx, ta.EntityCode, inv.ID); e != nil {
		t.Fatal(e)
	}
	if CheckSession(ctx, r, sid, "admin", inv.AccountID) == nil {
		t.Fatal("restored stale session")
	}
	var enabled string
	if e = s.DB.QueryRowCtx(ctx, &enabled, "SELECT status FROM admin_account WHERE id=?", inv.AccountID); e != nil || enabled != "enabled" {
		t.Fatalf("restore account: %s %v", enabled, e)
	}

}
