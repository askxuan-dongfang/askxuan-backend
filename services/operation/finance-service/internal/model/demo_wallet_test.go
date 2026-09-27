package model

import (
	"context"
	"github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestDemoFinanceMySQL(t *testing.T) {
	dsn := os.Getenv("FINANCE_DEMO_TEST_DSN")
	if dsn == "" || os.Getenv("WALLET_TEST_ISOLATED") != "1" {
		t.Skip("dedicated MySQL not configured")
	}
	logx.Disable()
	ctx := context.Background()
	cfg, e := mysql.ParseDSN(dsn)
	if e != nil {
		t.Fatal(e)
	}
	setup := sqlx.NewMysql(dsn)
	if _, e = setup.ExecCtx(ctx, `CREATE DATABASE askxuan_finance`); e != nil {
		t.Fatal(e)
	}
	cfg.DBName = "askxuan_finance"
	old := db
	Configure(sqlx.NewMysql(cfg.FormatDSN()))
	defer func() { db = old }()
	init, e := os.ReadFile("../../../../../db/init.sql")
	if e != nil {
		t.Fatal(e)
	}
	for _, table := range []string{"settlement", "withdrawal", "finance_log", "finance_transaction", "finance_ledger_entry"} {
		pattern := "(?s)CREATE TABLE IF NOT EXISTS `" + table + "` \\(.*?;"
		ddl := regexp.MustCompile(pattern).FindString(string(init))
		if ddl == "" {
			t.Fatal("missing table", table)
		}
		if _, e = db.ExecCtx(ctx, ddl); e != nil {
			t.Fatal(e)
		}
		if _, e = db.ExecCtx(ctx, "CREATE TABLE demo_"+table+" LIKE "+table); e != nil {
			t.Fatal(e)
		}
	}
	for _, q := range []string{
		`CREATE TABLE commission_config(biz_type VARCHAR(32) PRIMARY KEY,rate DECIMAL(5,4))`,
		`INSERT INTO commission_config VALUES('booking',0.15),('wild_master',0.15),('consultation',0.15)`,
		`CREATE DATABASE IF NOT EXISTS askxuan_payment`,
		`CREATE TABLE IF NOT EXISTS askxuan_payment.payment(id BIGINT AUTO_INCREMENT PRIMARY KEY,order_type VARCHAR(32),order_no VARCHAR(64),channel VARCHAR(16),status VARCHAR(24))`,
		`INSERT INTO askxuan_payment.payment(order_type,order_no,channel,status) VALUES('booking','DEMO-TEMPLE','demo_balance','success'),('booking','DEMO-SPLIT','demo_balance','success'),('booking','DEMO-REFUND','demo_balance','refunded')`,
	} {
		if _, e = db.ExecCtx(ctx, q); e != nil {
			t.Fatal(e)
		}
	}
	for _, no := range []string{"DEMO-TEMPLE", "DEMO-SPLIT", "DEMO-REFUND"} {
		if e = RecordPlatformReceipt(ctx, PaymentReceipt{PaymentNo: "P-" + no, SourceType: "booking", SourceNo: no, Amount: 100}); e != nil {
			t.Fatal(e)
		}
	}
	b := BookingSettlement{BookingID: "DEMO-TEMPLE", TempleID: "T-DEMO", BookingDate: "2026-09-28", ServiceFee: 70, MeritMoney: 30, TotalFee: 100}
	split, e := AccrueBookingSettlement(ctx, b)
	if e != nil {
		t.Fatal(e)
	}
	if !split.Simulated || split.MasterNet != 0 || split.TempleNet != 85 || split.Commission != 15 {
		t.Fatalf("whole temple allocation: %+v", split)
	}
	if _, e = AccrueBookingSettlement(ctx, b); e != nil {
		t.Fatal("retry", e)
	}
	if e = SimulatePayout(ctx, "temple", "T-FOREIGN", split.TempleSettlementID); e == nil {
		t.Fatal("foreign payout accepted")
	}
	// Inject a storage failure after withdrawal/ledger writes to verify rollback.
	if _, e = db.ExecCtx(ctx, `CREATE TRIGGER demo_payout_failure BEFORE UPDATE ON demo_settlement FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='isolated injected failure'`); e != nil {
		t.Fatal(e)
	}
	if e = SimulatePayout(ctx, "temple", "T-DEMO", split.TempleSettlementID); e == nil {
		t.Fatal("failure injection did not fail")
	}
	var failedCount int64
	if e = db.QueryRowCtx(ctx, &failedCount, `SELECT COUNT(*) FROM demo_withdrawal`); e != nil || failedCount != 0 {
		t.Fatal("failed payout left withdrawal", failedCount, e)
	}
	if _, e = db.ExecCtx(ctx, `DROP TRIGGER demo_payout_failure`); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		if e = SimulatePayout(ctx, "temple", "T-DEMO", split.TempleSettlementID); e != nil {
			t.Fatal(e)
		}
	}
	w, e := ReadProviderWallet(DemoLedgerContext(ctx), "temple", "T-DEMO", 1)
	if e != nil || w.Total != 1 || w.WithdrawalTotal != 1 || w.Summary.RecordedPaidCents != 8500 {
		t.Fatalf("payout not once: %+v %v", w, e)
	}
	b.BookingID = "DEMO-SPLIT"
	b.MasterID = "M-DEMO"
	split, e = AccrueBookingSettlement(ctx, b)
	if e != nil {
		t.Fatal(e)
	}
	if split.MasterNet != 59.5 || split.TempleNet != 25.5 {
		t.Fatalf("split: %+v", split)
	}
	if e = RecordPlatformRefund(ctx, PaymentReceipt{PaymentNo: "P-DEMO-REFUND", SourceType: "booking", SourceNo: "DEMO-REFUND", Amount: 100}); e != nil {
		t.Fatal(e)
	}
	b.BookingID = "DEMO-REFUND"
	if _, e = AccrueBookingSettlement(ctx, b); e == nil {
		t.Fatal("refunded payment settled")
	}
	for _, table := range []string{"finance_transaction", "finance_ledger_entry", "finance_log", "settlement", "withdrawal"} {
		var n int64
		if e = db.QueryRowCtx(ctx, &n, "SELECT COUNT(*) FROM "+table); e != nil || n != 0 {
			t.Fatal("real ledger polluted", table, n, e)
		}
	}
	var imbalance int64
	if e = db.QueryRowCtx(ctx, &imbalance, `SELECT COUNT(*) FROM (SELECT transaction_id FROM demo_finance_ledger_entry GROUP BY transaction_id HAVING ABS(SUM(IF(direction='debit',amount,-amount)))>0.001) x`); e != nil || imbalance != 0 {
		t.Fatal("ledger not balanced", imbalance, e)
	}
	overview, e := ReadDemoPlatformWallet(ctx)
	if e != nil || !strings.Contains(strings.TrimSpace(overview["mode"].(string)), "demo") {
		t.Fatal("overview", e)
	}
}
