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

func TestTempleAffiliationIntegration(t *testing.T) {
	dsn := os.Getenv("IDENTITY_TEST_MYSQL")
	if dsn == "" {
		t.Skip("requires isolated affiliation fixture")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.DBName = "askxuan_temple"
	db := sqlx.NewMysql(cfg.FormatDSN())
	ctx := context.Background()
	m := NewTempleModel(db)
	directory := NewDirectoryTagModel(db)
	b := make([]byte, 5)
	rand.Read(b)
	prefix := "at" + hex.EncodeToString(b)
	codes := []string{}
	tags := []string{}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.ExecCtx(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, code := range codes {
			db.ExecCtx(ctx, "DELETE FROM provider_directory_tag WHERE provider_kind='temple' AND provider_code=?", code)
			db.ExecCtx(ctx, "DELETE FROM temple WHERE code=?", code)
		}
		for _, code := range tags {
			db.ExecCtx(ctx, "DELETE FROM directory_tag WHERE code=?", code)
		}
	})
	tag := func(suffix, parent, status string) string {
		t.Helper()
		code := prefix + suffix
		exec("INSERT INTO directory_tag(code,tag_group,belief_code,name,value,status) VALUES (?,'tradition',?,?,?,?)", code, parent, code, code, status)
		tags = append(tags, code)
		return code
	}
	folk := tag("folk", "folk", "enabled")
	folk2 := tag("folk2", "folk", "enabled")
	disabled := tag("off", "folk", "disabled")
	cross := tag("cross", "daoism", "enabled")
	for _, suffix := range []string{"a", "b", "c", "d"} {
		code := prefix + suffix
		codes = append(codes, code)
		_, err := m.Insert(ctx, &Temple{Code: code, Name: "测试寺院", Region: "测试地区", Type: "民间信仰", BeliefCode: "folk", Status: TempleStatusNormal})
		if err != nil {
			t.Fatal(err)
		}
		if suffix != "d" {
			if err := directory.Assign(ctx, "temple", code, []string{folk, folk2}); err != nil {
				t.Fatal(err)
			}
		}
	}
	first := codes[0]
	exec("INSERT INTO provider_directory_tag(provider_kind,provider_code,tag_code) VALUES ('temple',?,?),('temple',?,?)", first, disabled, first, cross)
	assertRows := func(want int) {
		t.Helper()
		rows, err := directory.Assigned(ctx, "temple", first)
		if err != nil || len(rows) != want {
			t.Fatalf("relations %#v want %d %v", rows, want, err)
		}
	}
	assertRows(4)
	if err := directory.Assign(ctx, "temple", first, nil); err != nil {
		t.Fatal(err)
	}
	assertRows(4)
	if err := directory.Assign(ctx, "temple", first, []string{folk, folk2, disabled, cross}); err != nil {
		t.Fatal("retained historical labels rejected", err)
	}
	if err := directory.Assign(ctx, "temple", codes[3], []string{disabled}); err == nil {
		t.Fatal("new disabled association accepted")
	}
	if err := directory.Assign(ctx, "temple", codes[3], []string{cross}); err == nil {
		t.Fatal("new cross-parent association accepted")
	}
	if err := directory.Assign(ctx, "temple", first, []string{folk, "missing_fixture_tag"}); err == nil {
		t.Fatal("new unknown association accepted")
	}
	assertRows(4)
	entity, err := m.FindOne(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	entity.Sect = "旧文字补充"
	if err = m.Update(ctx, entity); err != nil {
		t.Fatal(err)
	}
	assertRows(4)
	entity.BeliefCode = "daoism"
	entity.Sect = "must not persist"
	if err = m.Update(ctx, entity); err == nil {
		t.Fatal("conflicting primary belief accepted")
	}
	entity, err = m.FindOne(ctx, first)
	if err != nil || entity.BeliefCode != "folk" || entity.Sect != "旧文字补充" {
		t.Fatalf("partial temple update %+v %v", entity, err)
	}
	for page := 1; page <= 4; page++ {
		rows, total, err := m.FindList(ctx, TempleFilter{BeliefCode: "folk", TraditionCode: folk}, page, 1)
		if err != nil {
			t.Fatal(err)
		}
		want := 1
		if page == 4 {
			want = 0
		}
		if total != 3 || len(rows) != want {
			t.Fatalf("page=%d rows=%d total=%d", page, len(rows), total)
		}
		for _, row := range rows {
			if !strings.HasPrefix(row.Code, prefix) || !strings.Contains(row.TraditionCodes, folk) {
				t.Fatalf("unfiltered row %+v", row)
			}
		}
	}
	// Public choices contain enabled tags; administrator choices retain disabled history.
	public, err := directory.List(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := directory.List(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	publicOff, adminOff := false, false
	for _, row := range public {
		publicOff = publicOff || row.Code == disabled
	}
	for _, row := range admin {
		adminOff = adminOff || row.Code == disabled
	}
	if publicOff || !adminOff {
		t.Fatal("directory visibility inconsistent")
	}
	p := &DirectoryTag{Code: folk, Group: "tradition", BeliefCode: "daoism", Name: folk, Value: folk, Status: "enabled"}
	if err := directory.Save(ctx, p, false); err == nil {
		t.Fatal("reparented associated tag")
	}
	p.BeliefCode = ""
	p.Name = "只改名保留主类"
	if err := directory.Save(ctx, p, false); err != nil {
		t.Fatal(err)
	}
	if p.BeliefCode != "folk" {
		t.Fatal("rename erased tag parent")
	}
	if err := directory.Assign(ctx, "temple", first, []string{}); err != nil {
		t.Fatal(err)
	}
	assertRows(0)
}
