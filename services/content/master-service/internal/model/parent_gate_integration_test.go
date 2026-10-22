package model

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestParentTempleNewBusinessGateMySQL(t *testing.T) {
	dsn := os.Getenv("ACCOUNT_SYSTEM_TEST_DSN")
	if dsn == "" || os.Getenv("ACCOUNT_SYSTEM_TEST_ISOLATED") != "1" {
		t.Skip("requires dedicated disposable account-system MySQL")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil || cfg.DBName != "askxuan_account_test" {
		t.Fatal("refusing non-test database")
	}
	sqlx.DisableLog()
	ctx := context.Background()
	db := sqlx.NewMysql(dsn)
	var count int64
	if db.QueryRowCtx(ctx, &count, `SELECT COUNT(*) FROM askxuan_temple.temple WHERE code='T950001'`) != nil || count != 0 {
		t.Fatal("fixture unavailable or occupied")
	}
	if _, err = db.ExecCtx(ctx, `INSERT INTO askxuan_temple.temple(id,code,name,status,auth_status,platform_status,shelf_status) VALUES(950001,'T950001','Fixture parent','正常','certified','normal','on_shelf')`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := db.ExecCtx(context.Background(), `DELETE FROM askxuan_temple.temple WHERE code='T950001'`); err != nil {
			t.Error(err)
		}
	})
	m := NewMasterModel(sqlx.NewMysql(strings.Replace(dsn, "/askxuan_account_test", "/askxuan_master", 1)))
	for _, tc := range []struct {
		name, auth, platform, shelf string
		want                        bool
	}{
		{"normal parent", "certified", "normal", "on_shelf", true},
		{"parent display shelf does not hide independently listed master", "certified", "normal", "off_shelf", true},
		{"platform ban stops new master orders", "certified", "banned", "off_shelf", false},
		{"pending parent stops new master orders", "pending", "normal", "off_shelf", false},
		{"unknown historical certification stays blocked", "legacy_unknown", "normal", "off_shelf", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := db.ExecCtx(ctx, `UPDATE askxuan_temple.temple SET auth_status=?,platform_status=?,shelf_status=? WHERE code='T950001'`, tc.auth, tc.platform, tc.shelf); err != nil {
				t.Fatal(err)
			}
			allowed, err := m.ParentAllowsNewBusiness(ctx, "T950001")
			if err != nil || allowed != tc.want {
				t.Fatalf("wrong parent helper allowed=%v err=%v", allowed, err)
			}
			var visible int64
			if err := db.QueryRowCtx(ctx, &visible, `SELECT COUNT(*) FROM (SELECT 'T950001' temple_code) master WHERE `+masterParentNewBusinessSQL); err != nil || (visible == 1) != tc.want {
				t.Fatalf("public SQL differs from RPC eligibility: %d %v", visible, err)
			}
		})
	}
	for _, tc := range []struct {
		code string
		want bool
	}{{"", true}, {"missing", false}} {
		ok, err := m.ParentAllowsNewBusiness(ctx, tc.code)
		if err != nil || ok != tc.want {
			t.Fatalf("unbound or missing parent behavior wrong: %q %v %v", tc.code, ok, err)
		}
	}
}
