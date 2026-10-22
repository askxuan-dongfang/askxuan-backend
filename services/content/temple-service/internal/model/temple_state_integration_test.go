package model

import (
	"context"
	"github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"sync"
	"sync/atomic"
	"testing"
)

func TestTempleStateIntegration(t *testing.T) {
	dsn := os.Getenv("ACCOUNT_SYSTEM_TEST_DSN")
	if dsn == "" || os.Getenv("ACCOUNT_SYSTEM_TEST_ISOLATED") != "1" {
		t.Skip("isolated account fixture required")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil || cfg.DBName != "askxuan_account_test" {
		t.Fatal("refusing non-isolated fixture")
	}
	cfg.DBName = "askxuan_temple"
	db := sqlx.NewMysql(cfg.FormatDSN())
	ctx := context.Background()
	sqlx.DisableLog()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.ExecCtx(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	for _, col := range []struct{ name, ddl string }{{"recommended", "TINYINT NOT NULL DEFAULT 0"}, {"state_version", "BIGINT NOT NULL DEFAULT 0"}} {
		var n int
		if err := db.QueryRowCtx(ctx, &n, "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema='askxuan_temple' AND table_name='temple' AND column_name=?", col.name); err != nil {
			t.Fatal(err)
		}
		if n == 0 {
			exec("ALTER TABLE temple ADD COLUMN " + col.name + " " + col.ddl)
		}
	}
	const code = "TSTATEP0"
	exec("DELETE FROM temple WHERE code=?", code)
	exec("DELETE FROM askxuan_auth.admin_account WHERE id IN (940001,940002)")
	exec("INSERT INTO temple(id,code,name,status,auth_status,platform_status,shelf_status) VALUES(940001,?,'状态测试','正常','certified','normal','on_shelf')", code)
	exec("INSERT INTO askxuan_auth.admin_account(id,account,name,role_id,temple_id,status) VALUES(940001,'state-t-a','寺院',1,?,'enabled'),(940002,'state-t-b','纳管师傅',2,?,'enabled')", code, code)
	t.Cleanup(func() {
		db.ExecCtx(ctx, "DELETE FROM askxuan_auth.admin_account WHERE id IN (940001,940002)")
		db.ExecCtx(ctx, "DELETE FROM temple WHERE code=?", code)
	})
	off, on, ban, normal := "off_shelf", "on_shelf", "banned", "normal"
	readEpoch := func() int64 {
		var v int64
		if err := db.QueryRowCtx(ctx, &v, "SELECT SUM(authz_version) FROM askxuan_auth.admin_account WHERE temple_id=?", code); err != nil {
			t.Fatal(err)
		}
		return v
	}
	s, err := ChangeTempleState(ctx, db, code, TempleStatePatch{Version: 0, ShelfStatus: &off})
	if err != nil {
		t.Fatal(err)
	}
	if IsTemplePublic(s) || s.PlatformStatus != "normal" || s.AuthStatus != "certified" || readEpoch() != 0 {
		t.Fatal("off-shelf changed work access")
	}
	s, err = ChangeTempleState(ctx, db, code, TempleStatePatch{Version: s.StateVersion, PlatformStatus: &ban})
	if err != nil {
		t.Fatal(err)
	}
	if s.Status != TempleStatusBanned || readEpoch() != 2 {
		t.Fatal("ban failed to revoke both work accounts")
	}
	s, err = ChangeTempleState(ctx, db, code, TempleStatePatch{Version: s.StateVersion, PlatformStatus: &normal})
	if err != nil {
		t.Fatal(err)
	}
	if s.ShelfStatus != off || readEpoch() != 4 {
		t.Fatal("unban revived old state or sessions")
	}
	if _, err = ChangeTempleState(ctx, db, code, TempleStatePatch{Version: 0, ShelfStatus: &on}); err == nil {
		t.Fatal("stale write accepted")
	}
	var success int32
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, e := ChangeTempleState(ctx, db, code, TempleStatePatch{Version: s.StateVersion, ShelfStatus: &on}); e == nil {
				atomic.AddInt32(&success, 1)
			}
		}()
	}
	wg.Wait()
	if success != 1 {
		t.Fatalf("concurrent CAS successes=%d", success)
	}
	exec("UPDATE temple SET auth_status='pending',shelf_status='off_shelf',state_version=10 WHERE code=?", code)
	if _, err = ChangeTempleState(ctx, db, code, TempleStatePatch{Version: 10, ShelfStatus: &on}); err == nil {
		t.Fatal("pending temple exposed")
	}
	if _, err = ChangeTempleState(ctx, db, "NOTEXISTS", TempleStatePatch{Version: 0, PlatformStatus: &ban}); err == nil {
		t.Fatal("missing temple accepted")
	}
	// Preserve pending certification through ban and unban, including the legacy API.
	m := NewTempleModel(db)
	if err = m.UpdateStatus(ctx, 940001, TempleStatusBanned); err != nil {
		t.Fatal(err)
	}
	if err = m.UpdateStatus(ctx, 940001, TempleStatusNormal); err == nil {
		t.Fatal("legacy unban bypassed audit")
	}
}
