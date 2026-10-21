package model

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestMasterAffiliationIntegration(t *testing.T) {
	dsn := os.Getenv("IDENTITY_TEST_MYSQL")
	if dsn == "" {
		t.Skip("requires isolated affiliation fixture")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.DBName = "askxuan_master"
	db := sqlx.NewMysql(cfg.FormatDSN())
	ctx := context.Background()
	m := NewMasterModel(db)
	b := make([]byte, 5)
	rand.Read(b)
	prefix := "am" + hex.EncodeToString(b)
	tags := []string{}
	codes := []string{}
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
	t.Cleanup(func() {
		for _, code := range codes {
			db.ExecCtx(ctx, "DELETE FROM askxuan_temple.provider_directory_tag WHERE provider_kind='master' AND provider_code=?", code)
			db.ExecCtx(ctx, "DELETE FROM master_audit WHERE master_code=?", code)
			db.ExecCtx(ctx, "DELETE FROM master WHERE code=?", code)
		}
		for _, code := range tags {
			db.ExecCtx(ctx, "DELETE FROM askxuan_temple.directory_tag WHERE code=?", code)
		}
	})
	tag := func(suffix, parent, status string) string {
		t.Helper()
		code := prefix + suffix
		exec("INSERT INTO askxuan_temple.directory_tag(code,tag_group,belief_code,name,value,status) VALUES (?,'tradition',?,?,?,?)", code, parent, code, code, status)
		tags = append(tags, code)
		return code
	}
	folk := tag("folk", "folk", "enabled")
	folk2 := tag("folk2", "folk", "enabled")
	disabled := tag("off", "folk", "disabled")
	cross := tag("cross", "daoism", "enabled")
	failed := tag("fail", "folk", "enabled")
	newMaster := func(suffix string) *Master {
		code := prefix + suffix
		codes = append(codes, code)
		return &Master{Code: code, DharmaName: "测试师傅", Position: "测试", BeliefCode: "folk", Type: "民间信仰", AuthStatus: MasterAuthStatusPending, ShelfStatus: MasterShelfStatusOffShelf, PlatformStatus: MasterPlatformStatusNormal, ManageBy: "platform", ConsultValidHours: 72, ConsultResponseMinutes: 30}
	}
	first := newMaster("a")
	first.Id, err = m.InsertWithAffiliation(ctx, first, []string{folk, folk2})
	if err != nil {
		t.Fatal(err)
	}
	if count("SELECT COUNT(*) FROM master_audit WHERE master_code=?", first.Code) != 1 || count("SELECT COUNT(*) FROM askxuan_temple.provider_directory_tag WHERE provider_code=?", first.Code) != 2 {
		t.Fatal("new profile/audit/relations incomplete")
	}
	for i, selected := range [][]string{nil, {folk, folk}, {disabled}, {cross}, {"missing_test_tag"}} {
		candidate := newMaster(string(rune('f' + i)))
		if _, err := m.InsertWithAffiliation(ctx, candidate, selected); err == nil {
			t.Fatalf("invalid create accepted %#v", selected)
		}
		if count("SELECT COUNT(*) FROM master WHERE code=?", candidate.Code) != 0 {
			t.Fatal("failed create leaked entity")
		}
	}
	// Historical multiple, disabled and cross-parent rows must survive text edits.
	exec("INSERT INTO askxuan_temple.provider_directory_tag(provider_kind,provider_code,tag_code) VALUES ('master',?,?),('master',?,?)", first.Code, disabled, first.Code, cross)
	first.Sect = "历史补充原文"
	if err := m.UpdateWithAffiliation(ctx, first, nil); err != nil {
		t.Fatal(err)
	}
	if count("SELECT COUNT(*) FROM askxuan_temple.provider_directory_tag WHERE provider_code=?", first.Code) != 4 {
		t.Fatal("text edit removed historical affiliations")
	}
	if err := m.UpdateWithAffiliation(ctx, first, []string{folk, folk2, disabled, cross}); err != nil {
		t.Fatal("retained history rejected", err)
	}
	first.BeliefCode = "daoism"
	if err := m.UpdateWithAffiliation(ctx, first, nil); err == nil {
		t.Fatal("conflicting belief change accepted")
	}
	first.BeliefCode = "folk"
	// Reject an addition atomically, preserving both text and the prior rows.
	first.Sect = "must not persist"
	if err := m.UpdateWithAffiliation(ctx, first, []string{folk, "missing_test_tag"}); err == nil {
		t.Fatal("unknown addition accepted")
	}
	var sect string
	if err := db.QueryRowCtx(ctx, &sect, "SELECT sect FROM master WHERE code=?", first.Code); err != nil || sect != "历史补充原文" {
		t.Fatalf("partial update %q %v", sect, err)
	}
	if count("SELECT COUNT(*) FROM askxuan_temple.provider_directory_tag WHERE provider_code=?", first.Code) != 4 {
		t.Fatal("failed update removed history")
	}
	// Empty supplemental text is allowed, with relationships left in place.
	first.Sect = ""
	if err := m.UpdateWithAffiliation(ctx, first, nil); err != nil {
		t.Fatal(err)
	}
	// Standard filtering is performed before count and page, across all statuses.
	for _, suffix := range []string{"b", "c"} {
		candidate := newMaster(suffix)
		candidate.Id, err = m.InsertWithAffiliation(ctx, candidate, []string{folk})
		if err != nil {
			t.Fatal(err)
		}
	}
	for page := 1; page <= 4; page++ {
		rows, total, err := m.FindPlatformList(ctx, "folk", "", "", "", "", "", "", "", folk, page, 1)
		if err != nil {
			t.Fatal(err)
		}
		want := 1
		if page == 4 {
			want = 0
		}
		if total != 3 || len(rows) != want {
			t.Fatalf("page %d rows=%d total=%d", page, len(rows), total)
		}
		for _, row := range rows {
			if !strings.HasPrefix(row.Code, prefix) || !strings.Contains(row.TraditionCodes, folk) {
				t.Fatalf("unexpected filtered row %+v", row)
			}
		}
	}
	// A self-service request read before an administrator's atomic reclassification
	// must not restore its stale belief or supplemental text when changing avatar.
	stale := *first
	first.BeliefCode = "daoism"
	first.Sect = "管理员新说明"
	if err := m.UpdateWithAffiliation(ctx, first, []string{cross}); err != nil {
		t.Fatal(err)
	}
	stale.Avatar = "https://fixture.invalid/new-avatar.png"
	if err := m.UpdateWorkspaceProfile(ctx, &stale); err != nil {
		t.Fatal(err)
	}
	latest, err := m.FindByCode(ctx, first.Code)
	if err != nil || latest.BeliefCode != "daoism" || latest.Sect != "管理员新说明" || latest.TraditionCodes != cross || latest.Avatar != stale.Avatar {
		t.Fatalf("stale self-service update changed standard affiliation: %+v %v", latest, err)
	}
	// Failure after the entity and qualification row were inserted rolls them back.
	trigger := prefix + "_reject"
	exec("CREATE TRIGGER askxuan_temple." + trigger + " BEFORE INSERT ON askxuan_temple.provider_directory_tag FOR EACH ROW BEGIN IF NEW.tag_code='" + failed + "' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fixture relation failure'; END IF; END")
	defer db.ExecCtx(ctx, "DROP TRIGGER askxuan_temple."+trigger)
	candidate := newMaster("x")
	if _, err := m.InsertWithAffiliation(ctx, candidate, []string{failed}); err == nil {
		t.Fatal("association failure ignored")
	}
	if count("SELECT COUNT(*) FROM master WHERE code=?", candidate.Code) != 0 || count("SELECT COUNT(*) FROM master_audit WHERE master_code=?", candidate.Code) != 0 {
		t.Fatal("association failure leaked master or qualification")
	}
}
