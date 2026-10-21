package identity

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// This test owns only uniquely named rows and its one conditional trigger in the
// explicitly configured isolated fixture. No catalog seed or existing rows change.
func TestAffiliationOnboardingIntegration(t *testing.T) {
	dsn, host := os.Getenv("IDENTITY_TEST_MYSQL"), os.Getenv("IDENTITY_TEST_REDIS")
	if dsn == "" || host == "" {
		t.Skip("requires isolated MySQL/Redis affiliation fixture")
	}
	ctx := context.Background()
	db := sqlx.NewMysql(dsn)
	s := Accounts{DB: db, Challenges: &Challenges{Redis: redis.MustNewRedis(redis.RedisConf{Host: host, Type: "node"}), Secret: RandomID()}}
	prefix := "af" + strings.ToLower(RandomID()[:10])
	tags := []string{}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.ExecCtx(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	count := func(q string, args ...any) int64 {
		t.Helper()
		var n int64
		if err := db.QueryRowCtx(ctx, &n, q, args...); err != nil {
			t.Fatal(err)
		}
		return n
	}
	accounts := []int64{}
	apps := []int64{}
	entities := []string{}
	t.Cleanup(func() {
		for _, id := range apps {
			db.ExecCtx(ctx, "DELETE FROM onboarding_event WHERE application_id=?", id)
		}
		for _, id := range accounts {
			db.ExecCtx(ctx, "DELETE FROM onboarding_evidence WHERE account_id=?", id)
			db.ExecCtx(ctx, "DELETE FROM onboarding_application WHERE account_id=?", id)
			db.ExecCtx(ctx, "DELETE FROM askxuan_temple.temple_admin WHERE account_id=?", id)
			db.ExecCtx(ctx, "DELETE FROM admin_account WHERE id=?", id)
		}
		for _, code := range entities {
			db.ExecCtx(ctx, "DELETE FROM askxuan_temple.provider_directory_tag WHERE provider_code=?", code)
			db.ExecCtx(ctx, "DELETE FROM askxuan_master.master_profile_ext WHERE master_code=?", code)
			db.ExecCtx(ctx, "DELETE FROM askxuan_master.master WHERE code=?", code)
			db.ExecCtx(ctx, "DELETE FROM askxuan_temple.temple WHERE code=?", code)
		}
		for _, code := range tags {
			db.ExecCtx(ctx, "DELETE FROM askxuan_temple.directory_tag WHERE code=?", code)
		}
	})
	tag := func(suffix, belief, status string) string {
		t.Helper()
		code := prefix + suffix
		exec("INSERT INTO askxuan_temple.directory_tag(code,tag_group,belief_code,name,value,status) VALUES (?,'tradition',?,?,?,?)", code, belief, code, code, status)
		tags = append(tags, code)
		return code
	}
	han := tag("han", "han_buddhism", "enabled")
	disabled := tag("off", "han_buddhism", "disabled")
	folk := tag("folk", "folk", "enabled")
	folk2 := tag("folk2", "folk", "enabled")
	fail := tag("fail", "han_buddhism", "enabled")
	create := func(kind string) (int64, WorkProfile, *Application) {
		t.Helper()
		name := prefix + RandomID()[:6]
		res, err := db.ExecCtx(ctx, "INSERT INTO admin_account(account,password,name,role_id,status) SELECT ?,'unused',?,id,'enabled' FROM role WHERE code=?", name, name, ApplicantRole(kind))
		if err != nil {
			t.Fatal(err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			t.Fatal(err)
		}
		accounts = append(accounts, id)
		exec("INSERT INTO onboarding_application(account_id,kind,profile_json) VALUES (?,?,'{}')", id, kind)
		evidence := RandomID()
		exec("INSERT INTO onboarding_evidence(id,account_id,filename,media_type,content) VALUES (?,?,'test.pdf','application/pdf',?)", evidence, id, []byte("fixture"))
		p := WorkProfile{Name: name, LegalName: "测试本人", Contact: "测试联系", Region: "测试地区", Address: "测试地址", RegistrationNumber: "fixture", Position: "测试职务", Belief: "han_buddhism", TraditionCodes: []string{han}, EvidenceIDs: []string{evidence}}
		a, err := s.WorkApplication(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		apps = append(apps, a.ID)
		return id, p, a
	}
	t.Run("draft_submit_and_strict_new_tags", func(t *testing.T) {
		id, p, a := create("master")
		p.TraditionCodes = []string{}
		var err error
		a, err = s.SaveWorkApplication(ctx, id, p, a.Revision, false)
		if err != nil {
			t.Fatal(err)
		}
		for _, codes := range [][]string{nil, {}, {disabled}, {folk}, {han, han}, {"missing_fixture"}} {
			p.TraditionCodes = codes
			if _, err := s.SaveWorkApplication(ctx, id, p, a.Revision, true); err == nil {
				t.Fatalf("accepted tags %#v", codes)
			}
		}
		p.TraditionCodes = []string{han}
		a, err = s.SaveWorkApplication(ctx, id, p, a.Revision, false)
		if err != nil {
			t.Fatal(err)
		}
		p.TraditionCodes = nil
		a, err = s.SaveWorkApplication(ctx, id, p, a.Revision, true)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.ReviewWorkApplication(ctx, 999999, a.ID, a.Revision, true, ""); err != nil {
			t.Fatal(err)
		}
		a, err = s.WorkApplication(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		entities = append(entities, a.EntityCode)
		if count("SELECT COUNT(*) FROM askxuan_temple.provider_directory_tag WHERE provider_kind='master' AND provider_code=? AND tag_code=?", a.EntityCode, han) != 1 {
			t.Fatal("missing approval relation")
		}
	})
	t.Run("five_types_alias_and_multiple_folk", func(t *testing.T) {
		for i, belief := range []string{"han_buddhism", "tibetan_buddhism", "theravada_buddhism", "daoism", "taoism", "folk"} {
			kind := "master"
			if i%2 == 0 {
				kind = "temple"
			}
			id, p, a := create(kind)
			canonical := belief
			if belief == "taoism" {
				canonical = "daoism"
			}
			p.Belief = belief
			p.TraditionCodes = []string{tag("case"+string(rune('a'+i)), canonical, "enabled")}
			if belief == "folk" {
				p.TraditionCodes = []string{folk, folk2}
			}
			var err error
			a, err = s.SaveWorkApplication(ctx, id, p, a.Revision, true)
			if err != nil {
				t.Fatalf("%s: %v", belief, err)
			}
			if err = s.ReviewWorkApplication(ctx, 999999, a.ID, a.Revision, true, ""); err != nil {
				t.Fatalf("%s: %v", belief, err)
			}
			a, err = s.WorkApplication(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			entities = append(entities, a.EntityCode)
			table := "askxuan_master.master"
			if kind == "temple" {
				table = "askxuan_temple.temple"
			}
			var got string
			if err = db.QueryRowCtx(ctx, &got, "SELECT belief_code FROM "+table+" WHERE code=?", a.EntityCode); err != nil || got != canonical {
				t.Fatalf("belief %s %v", got, err)
			}
			if count("SELECT COUNT(*) FROM askxuan_temple.provider_directory_tag WHERE provider_code=?", a.EntityCode) != int64(len(p.TraditionCodes)) {
				t.Fatal("incomplete relations")
			}
		}
	})
	t.Run("legacy_submission_requires_return_for_supplement", func(t *testing.T) {
		id, p, a := create("temple")
		p.TraditionCodes = nil
		p.Sect = "旧文字原文"
		raw, _ := json.Marshal(p)
		exec("UPDATE onboarding_application SET profile_json=?,status='submitted' WHERE id=?", string(raw), a.ID)
		if err := s.ReviewWorkApplication(ctx, 999999, a.ID, a.Revision, true, ""); err == nil {
			t.Fatal("approved legacy untagged submission")
		}
		a, err := s.WorkApplication(ctx, id)
		if err != nil || a.Status != "submitted" || a.EntityCode != "" {
			t.Fatalf("legacy state %+v %v", a, err)
		}
		if err := s.ReviewWorkApplication(ctx, 999999, a.ID, a.Revision, false, "请补充标准标签"); err != nil {
			t.Fatal(err)
		}
		a, err = s.WorkApplication(ctx, id)
		if err != nil || a.Status != "rejected" || !strings.Contains(a.ProfileJSON, "旧文字原文") {
			t.Fatal("legacy text or return state lost", err)
		}
	})
	t.Run("association_failure_rolls_back_all_databases", func(t *testing.T) {
		trigger := prefix + "_reject"
		exec("CREATE TRIGGER askxuan_temple." + trigger + " BEFORE INSERT ON askxuan_temple.provider_directory_tag FOR EACH ROW BEGIN IF NEW.tag_code='" + fail + "' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fixture association failure'; END IF; END")
		defer db.ExecCtx(ctx, "DROP TRIGGER askxuan_temple."+trigger)
		for _, kind := range []string{"master", "temple"} {
			id, p, a := create(kind)
			p.TraditionCodes = []string{fail}
			var err error
			a, err = s.SaveWorkApplication(ctx, id, p, a.Revision, true)
			if err != nil {
				t.Fatal(err)
			}
			before := count("SELECT COUNT(*) FROM onboarding_event WHERE application_id=?", a.ID)
			if err = s.ReviewWorkApplication(ctx, 999999, a.ID, a.Revision, true, ""); err == nil {
				t.Fatal("injected association failure ignored")
			}
			after, err := s.WorkApplication(ctx, id)
			if err != nil || after.Status != "submitted" || after.Revision != a.Revision || after.EntityCode != "" {
				t.Fatalf("partial application %+v %v", after, err)
			}
			if count("SELECT COUNT(*) FROM onboarding_event WHERE application_id=?", a.ID) != before {
				t.Fatal("partial review event")
			}
			if count("SELECT COUNT(*) FROM admin_account a JOIN role r ON r.id=a.role_id WHERE a.id=? AND r.code=? AND a.temple_id='' AND a.master_id=''", id, ApplicantRole(kind)) != 1 {
				t.Fatal("partial account binding")
			}
			if count("SELECT COUNT(*) FROM askxuan_master.master WHERE dharma_name=?", p.Name)+count("SELECT COUNT(*) FROM askxuan_temple.temple WHERE name=?", p.Name) != 0 {
				t.Fatal("partial business entity")
			}
		}
	})
}
