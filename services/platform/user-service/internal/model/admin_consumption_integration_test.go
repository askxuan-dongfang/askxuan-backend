package model

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func accountTestDB(t *testing.T) sqlx.SqlConn {
	t.Helper()
	sqlx.DisableLog()
	dsn := os.Getenv("ACCOUNT_SYSTEM_TEST_DSN")
	if dsn == "" || os.Getenv("ACCOUNT_SYSTEM_TEST_ISOLATED") != "1" {
		t.Skip("requires a disposable MySQL with ACCOUNT_SYSTEM_TEST_DSN and ACCOUNT_SYSTEM_TEST_ISOLATED=1")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil || cfg.DBName != "askxuan_account_test" {
		t.Fatal("refusing non-test database")
	}
	db := sqlx.NewMysql(dsn)
	ctx := context.Background()
	var name string
	if db.QueryRowCtx(ctx, &name, "SELECT DATABASE()") != nil || name != cfg.DBName {
		t.Fatal("isolated database unavailable")
	}
	ddl, err := os.ReadFile("../../../../../scripts/dev/account-system/fixture.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range strings.Split(string(ddl), ";") {
		if strings.TrimSpace(q) != "" {
			if _, err = db.ExecCtx(ctx, q); err != nil {
				t.Fatal(err)
			}
		}
	}
	return db
}

func TestRealUserConsumptionMySQL(t *testing.T) {
	db := accountTestDB(t)
	ctx := context.Background()
	sqlx.DisableLog()
	const uid = 910001
	var existing int64
	if db.QueryRowCtx(ctx, &existing, `SELECT COUNT(*) FROM askxuan_payment.payment WHERE user_id=?`, uid) != nil || existing != 0 {
		t.Fatal("fixture rows already present; refusing overwrite")
	}
	t.Cleanup(func() {
		for _, q := range []string{`DELETE FROM askxuan_payment.refund WHERE id BETWEEN 910001 AND 910099`, `DELETE FROM askxuan_payment.payment WHERE id BETWEEN 910001 AND 910099`, `DELETE FROM askxuan_auth.auth_identity WHERE user_id=910001 AND domain IN ('user','admin')`, `DELETE FROM askxuan_user.user WHERE id=910001`} {
			if _, err := db.ExecCtx(context.Background(), q); err != nil {
				t.Error(err)
			}
		}
	})
	for _, q := range []string{
		`INSERT INTO askxuan_payment.payment VALUES (910001,'910001','booking','B-real',100.01,'wechat','success'),(910002,'910001','consultation','C-real',20,'alipay','refunding'),(910003,'910001','shop_order','S-full-refund',50,'balance','refunded'),(910004,'910001','diy_order','D-real',30,'balance','success'),(910005,'910001','ai_report','A-real',9.99,'wechat','success'),(910006,'910001','diy_order','EXO-demo',999,'balance','success'),(910007,'910001','diy_order','D-mock',999,'mock','success'),(910008,'910001','booking','B-demo',999,'demo_balance','success'),(910009,'910001','recharge','R-cash',999,'wechat','success'),(910010,'910001','booking','B-failed',999,'wechat','failed'),(910011,'910001','booking','B-pending',999,'wechat','pending'),(910012,'910001','booking','B-real',50,'wechat','pending'),(910013,'910001','booking','B-real',1,'wechat','success')`,
		`INSERT INTO askxuan_payment.refund VALUES (910001,910001,10,'success'),(910002,910001,5.01,'success'),(910003,910001,999,'failed'),(910004,910002,1,'processing'),(910005,910003,10,'success')`,
	} {
		if _, err := db.ExecCtx(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	stats, err := ReadUserConsumption(ctx, db, []int64{uid, 910002})
	if err != nil {
		t.Fatal(err)
	}
	if stats[uid].TotalOrders != 5 || stats[uid].SpentCents != 14599 {
		t.Fatalf("wrong count/net cents: %+v", stats)
	}
	if stats[910002].TotalOrders != 0 || stats[910002].SpentCents != 0 {
		t.Fatal("new user was not zero")
	}
	if _, err = ReadUserConsumption(ctx, db, []int64{-1}); err == nil {
		t.Fatal("negative id accepted")
	}
	t.Run("live successful refund changes totals without profile cache", func(t *testing.T) {
		if _, err := db.ExecCtx(ctx, `INSERT INTO askxuan_payment.refund VALUES(910006,910004,99,'success')`); err != nil {
			t.Fatal(err)
		}
		after, err := ReadUserConsumption(ctx, db, []int64{uid})
		if err != nil || after[uid].TotalOrders != 5 || after[uid].SpentCents != 11599 {
			t.Fatalf("net spending not live/clamped: %+v %v", after, err)
		}
	})
	t.Run("credential domains never merge", func(t *testing.T) {
		for _, q := range []string{`INSERT INTO askxuan_auth.auth_identity(domain,user_id,username,email,verified_at) VALUES('user',910001,'consumer_fixture','shared@example.test',NOW()),('admin',910001,'staff_fixture','other@example.test',NOW())`} {
			if _, err := db.ExecCtx(ctx, q); err != nil {
				t.Fatal(err)
			}
		}
		c, err := ReadUserCredential(ctx, db, uid)
		if err != nil || c.Email != "shared@example.test" || !c.EmailVerified || !c.CanResetPassword {
			t.Fatalf("wrong consumer identity: %+v %v", c, err)
		}
	})
	t.Run("status toggles advance version once and preserve profile", func(t *testing.T) {
		if _, err := db.ExecCtx(ctx, `INSERT INTO askxuan_user.user(id,nickname,status,authz_version) VALUES(910001,'fixture',1,0)`); err != nil {
			t.Fatal(err)
		}
		m := NewUserModel(sqlx.NewMysql(strings.Replace(os.Getenv("ACCOUNT_SYSTEM_TEST_DSN"), "/askxuan_account_test", "/askxuan_user", 1)))
		for _, status := range []int{0, 0, 1} {
			if err := m.UpdateStatus(ctx, uid, status); err != nil {
				t.Fatal(err)
			}
		}
		var row struct {
			Status  int    `db:"status"`
			Version int64  `db:"authz_version"`
			Name    string `db:"nickname"`
		}
		if err := db.QueryRowCtx(ctx, &row, `SELECT status,authz_version,nickname FROM askxuan_user.user WHERE id=910001`); err != nil || row.Status != 1 || row.Version != 2 || row.Name != "fixture" {
			t.Fatalf("status/version transition violated: %+v %v", row, err)
		}
	})
}
