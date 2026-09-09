package handler

import (
	"context"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"sync"
	"testing"
)

func TestMySQLReportPurchase(t *testing.T) {
	dsn := os.Getenv("REPORT_PAYMENT_TEST_DSN")
	if dsn == "" {
		t.Skip("isolated REPORT_PAYMENT_TEST_DSN not set")
	}
	db := sqlx.NewMysql(dsn)
	ctx := context.Background()
	var id int64
	if e := db.QueryRowCtx(ctx, &id, `SELECT id FROM askxuan_ai.ai_report WHERE user_id='9501' AND request_key='mysql-report-case'`); e != nil {
		t.Fatal(e)
	}
	if _, e := db.ExecCtx(ctx, `INSERT INTO points_account(user_id,balance) VALUES('9501',50)`); e != nil {
		t.Fatal(e)
	}
	if e := redeemReport(ctx, db, "9502", id, 10); e == nil {
		t.Fatal("other user bought report")
	}
	if e := redeemReport(ctx, db, "9501", id, 1); e == nil {
		t.Fatal("tampered price accepted")
	}
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- redeemReport(ctx, db, "9501", id, 10) }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	var balance, count int64
	_ = db.QueryRowCtx(ctx, &balance, `SELECT balance FROM points_account WHERE user_id='9501'`)
	_ = db.QueryRowCtx(ctx, &count, `SELECT COUNT(*) FROM points_ledger WHERE user_id='9501' AND kind='redeem'`)
	if balance != 40 || count != 1 {
		t.Fatalf("duplicate charge balance=%d entries=%d", balance, count)
	}
}
