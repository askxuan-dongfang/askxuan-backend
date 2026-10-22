package cashier

import (
	"context"
	"errors"
	"github.com/askxuan/payment-service/internal/balance"
	"github.com/askxuan/payment-service/internal/model"
	"github.com/askxuan/payment-service/internal/paychannel"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
)

// Requires a dedicated disposable MySQL instance, never a developer or production DB.
func TestCashWalletMySQL(t *testing.T) {
	dsn := os.Getenv("WALLET_TEST_DSN")
	if dsn == "" || os.Getenv("WALLET_TEST_ISOLATED") != "1" {
		t.Skip("isolated MySQL not configured")
	}
	logx.Disable()
	ctx := context.Background()
	db := sqlx.NewMysql(dsn)
	ddl := []string{
		`CREATE DATABASE askxuan_order`, `CREATE DATABASE askxuan_diy`, `CREATE DATABASE askxuan_booking`, `CREATE DATABASE askxuan_ai`, `CREATE DATABASE askxuan_temple`, `CREATE DATABASE askxuan_master`,
		`CREATE TABLE askxuan_temple.temple(code VARCHAR(32) PRIMARY KEY,auth_status VARCHAR(24),platform_status VARCHAR(16),shelf_status VARCHAR(16))`,
		`CREATE TABLE askxuan_master.master(code VARCHAR(32) PRIMARY KEY,temple_code VARCHAR(32),auth_status VARCHAR(24),platform_status VARCHAR(16),shelf_status VARCHAR(16))`,
		`INSERT INTO askxuan_temple.temple VALUES ('fixture','certified','normal','on_shelf')`,
		`INSERT INTO askxuan_master.master VALUES ('master','fixture','已认证','normal','on_shelf'),('other-master','','已认证','normal','on_shelf')`,
		`CREATE TABLE payment(id BIGINT PRIMARY KEY AUTO_INCREMENT,payment_no VARCHAR(64) UNIQUE,idempotency_key VARCHAR(128) UNIQUE,user_id VARCHAR(64),order_type VARCHAR(32),order_no VARCHAR(32) NOT NULL,amount DECIMAL(12,2),channel VARCHAR(16),status VARCHAR(24),trade_no VARCHAR(96),create_time DATETIME,update_time DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE refund(id BIGINT PRIMARY KEY AUTO_INCREMENT,refund_no VARCHAR(64) UNIQUE,payment_id BIGINT,amount DECIMAL(12,2),reason VARCHAR(255),status VARCHAR(24),create_time DATETIME)`,
		`CREATE TABLE points_payment_award(payment_id BIGINT PRIMARY KEY,user_id VARCHAR(64),awarded BIGINT,reversed BIGINT DEFAULT 0)`,
		`CREATE TABLE event_outbox(id BIGINT AUTO_INCREMENT PRIMARY KEY,event_key VARCHAR(128) UNIQUE,aggregate_type VARCHAR(32),aggregate_id VARCHAR(64),event_type VARCHAR(64),exchange_name VARCHAR(64),routing_key VARCHAR(64),payload JSON,status VARCHAR(24),retry_count INT,next_retry_at DATETIME,created_at DATETIME,updated_at DATETIME)`,
		`CREATE TABLE askxuan_order.shop_order(order_no VARCHAR(64) PRIMARY KEY,user_id VARCHAR(64),pay_amount DECIMAL(12,2),status VARCHAR(24))`,
		`CREATE TABLE askxuan_diy.diy_order(order_no VARCHAR(64) PRIMARY KEY,user_id VARCHAR(64),total_fee DECIMAL(12,2),status VARCHAR(24),payment_status VARCHAR(24),bless_fee DECIMAL(12,2) NOT NULL DEFAULT 0,pricing_snapshot TEXT)`,
		`CREATE TABLE askxuan_booking.booking(booking_no VARCHAR(64) PRIMARY KEY,user_id VARCHAR(64),total_fee DECIMAL(12,2),status VARCHAR(24),payment_status VARCHAR(24),payment_expire_time DATETIME,slot_reserved INT,slot_code VARCHAR(32),master_code VARCHAR(32),temple_code VARCHAR(32) NOT NULL DEFAULT 'fixture',payment_no VARCHAR(64),payment_channel VARCHAR(16),temple_name VARCHAR(64),master_name VARCHAR(64),service_name VARCHAR(64),booking_date DATE,service_fee DECIMAL(10,2),merit_money DECIMAL(10,2))`,
		`CREATE TABLE askxuan_booking.booking_status_log(id BIGINT AUTO_INCREMENT PRIMARY KEY,booking_id VARCHAR(64),from_status VARCHAR(24),to_status VARCHAR(24),operator_id VARCHAR(64),operator_type VARCHAR(32),remark VARCHAR(255),create_time DATETIME)`,
		`CREATE TABLE askxuan_booking.consultation_order(order_no VARCHAR(64) PRIMARY KEY,user_id VARCHAR(64),master_code VARCHAR(32),consult_fee DECIMAL(12,2),status VARCHAR(24),payment_status VARCHAR(24),payment_no VARCHAR(64),payment_channel VARCHAR(16),valid_from DATETIME,expires_at DATETIME,valid_hours INT,temple_code VARCHAR(32) NOT NULL DEFAULT '')`,
		`CREATE TABLE askxuan_ai.ai_report(report_no VARCHAR(64) PRIMARY KEY,user_id VARCHAR(64),price_cents BIGINT,points_paid INT,status VARCHAR(24))`,
	}
	for _, q := range ddl {
		if _, e := db.ExecCtx(ctx, q); e != nil {
			t.Fatal(e)
		}
	}
	// Start with the deployed 32-character column and apply the upgrade, rather
	// than hiding missing migrations behind a wider test-only schema.
	orderMigration, e := os.ReadFile("../../../../../scripts/db/20261020_ai_report_payment_order_no.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.ExecCtx(ctx, string(orderMigration)); e != nil {
		t.Fatal(e)
	}
	migration, e := os.ReadFile("../../../../../scripts/db/20260926_balance_wallet.sql")
	if e != nil {
		t.Fatal(e)
	}
	for _, q := range strings.Split(strings.Split(string(migration), "-- All databases")[0], ";") {
		if strings.TrimSpace(q) != "" {
			if _, e = db.ExecCtx(ctx, q); e != nil {
				t.Fatal(e)
			}
		}
	}
	t.Run("demo wallet isolation", func(t *testing.T) { checkDemoWallet(t, db) })
	s := Store{DB: db, Enabled: true, Mock: true, Channels: map[string]paychannel.Gateway{}}
	_, e = db.ExecCtx(ctx, `INSERT INTO wallet_recharge(recharge_no,user_id,request_id,channel,amount_cents,pay_url) VALUES('W-test','10','request-test-123456','wechat',10000,'')`)
	if e != nil {
		t.Fatal(e)
	}
	run := func(n int, fn func() error) []error {
		out := make([]error, n)
		var wg sync.WaitGroup
		for i := range out {
			wg.Add(1)
			go func(i int) { defer wg.Done(); out[i] = fn() }(i)
		}
		wg.Wait()
		return out
	}
	for _, e := range run(10, func() error {
		return s.Apply(ctx, "wechat", paychannel.Result{No: "W-test", TradeNo: "T-real-fixture", Cents: 10000, State: "success"})
	}) {
		if e != nil {
			t.Fatal(e)
		}
	}
	a, _ := balance.Read(ctx, db, "10")
	if a.Available != 10000 {
		t.Fatalf("callback credited twice: %+v", a)
	}
	if e = s.Apply(ctx, "wechat", paychannel.Result{No: "W-test", TradeNo: "other", Cents: 10000, State: "success"}); e == nil {
		t.Fatal("conflicting transaction accepted")
	}
	if e = s.Apply(ctx, "alipay", paychannel.Result{No: "W-test", TradeNo: "T-real-fixture", Cents: 10000, State: "success"}); e == nil {
		t.Fatal("foreign channel accepted")
	}
	if _, e = db.ExecCtx(ctx, `INSERT INTO askxuan_order.shop_order VALUES('SHOP-1','10',30,'pending_payment'),('SHOP-2','10',80,'pending_payment'),('SHOP-3','10',80,'pending_payment')`); e != nil {
		t.Fatal(e)
	}
	req := OrderRequest{OrderType: "shop_order", OrderNo: "SHOP-1", ExpectedCents: 3000, Channel: "balance"}
	if _, e = s.Pay(ctx, "11", req); e == nil {
		t.Fatal("foreign user charged")
	}
	bad := req
	bad.ExpectedCents = 1
	if _, e = s.Pay(ctx, "10", bad); e == nil {
		t.Fatal("client amount trusted")
	}
	for _, e := range run(10, func() error { _, e := s.Pay(ctx, "10", req); return e }) {
		if e != nil {
			t.Fatal(e)
		}
	}
	a, _ = balance.Read(ctx, db, "10")
	if a.Available != 7000 {
		t.Fatalf("duplicate spend %+v", a)
	}
	var paid string
	db.QueryRowCtx(ctx, &paid, `SELECT status FROM askxuan_order.shop_order WHERE order_no='SHOP-1'`)
	if paid != "paid" {
		t.Fatal("order not committed with debit")
	}
	if _, e = s.Pay(ctx, "10", OrderRequest{OrderType: "shop_order", OrderNo: "SHOP-2", ExpectedCents: 8000, Channel: "balance"}); e == nil {
		t.Fatal("overspend succeeded")
	}
	var count int
	db.QueryRowCtx(ctx, &count, `SELECT COUNT(*) FROM payment WHERE order_no='SHOP-2'`)
	if count != 0 {
		t.Fatal("failed debit left payment behind")
	}
	pm := model.NewPaymentModel(db)
	p, e := pm.FindByOrder(ctx, "shop_order", "SHOP-1")
	if e != nil {
		t.Fatal(e)
	}
	for _, e := range run(10, func() error { _, e := model.AtomicMockRefund(ctx, db, p.Id, 30, "test approved return"); return e }) {
		if e != nil {
			t.Fatal(e)
		}
	}
	a, _ = balance.Read(ctx, db, "10")
	if a.Available != 10000 {
		t.Fatalf("refund duplicated %+v", a)
	}
	// Two independent orders compete for the same cash. One must lose.
	var wg sync.WaitGroup
	results := make([]error, 2)
	for i, no := range []string{"SHOP-2", "SHOP-3"} {
		wg.Add(1)
		go func(i int, no string) {
			defer wg.Done()
			_, results[i] = s.Pay(ctx, "10", OrderRequest{OrderType: "shop_order", OrderNo: no, ExpectedCents: 8000, Channel: "balance"})
		}(i, no)
	}
	wg.Wait()
	success := 0
	for _, e := range results {
		if e == nil {
			success++
		}
	}
	for _, e := range results {
		if e != nil && e != balance.ErrFunds {
			t.Fatalf("expected insufficient funds, got %v", e)
		}
	}
	if success != 1 {
		t.Fatalf("concurrent balance invariant %v", results)
	}
	a, _ = balance.Read(ctx, db, "10")
	if a.Available != 2000 {
		t.Fatal(a)
	}
	// No merchant config can create or credit a recharge.
	if _, e = s.CreateRecharge(ctx, "10", "mock", "disabled-request-1234", "127.0.0.1", 1000); e == nil {
		t.Fatal("mock recharge accepted")
	}
	s.Channels["wechat"] = fixtureGateway{}
	f, e := s.RefundRecharge(ctx, "10", "W-test", 2000)
	if e != nil {
		t.Fatal(e)
	}
	a, _ = balance.Read(ctx, db, "10")
	if a.Available != 0 || a.Held != 2000 {
		t.Fatal("refund not frozen", a)
	}
	if _, e = s.RefundRecharge(ctx, "11", "W-test", 2000); e == nil {
		t.Fatal("foreign recharge refund accepted")
	}
	if _, e = s.RefundRecharge(ctx, "10", "W-test", 1000); e == nil {
		t.Fatal("retry changed refund amount")
	}
	if e = s.reconcileOne(ctx, f); e != nil {
		t.Fatal(e)
	}
	if e = s.reconcileOne(ctx, f); e != nil {
		t.Fatal(e)
	}
	a, _ = balance.Read(ctx, db, "10")
	if a.Available != 0 || a.Held != 0 {
		t.Fatal("held amount not settled once", a)
	}
	_, e = db.ExecCtx(ctx, `INSERT INTO askxuan_diy.diy_order(order_no,user_id,total_fee,status,payment_status) VALUES('D1','20',9.9,'pending_review','pending')`)
	if e != nil {
		t.Fatal(e)
	}
	_, e = db.ExecCtx(ctx, `INSERT INTO askxuan_booking.booking(booking_no,user_id,total_fee,status,payment_status,payment_expire_time,slot_reserved,slot_code,master_code,payment_no,payment_channel) VALUES('B1','20',9.9,'pending_payment','pending',DATE_ADD(NOW(),INTERVAL 15 MINUTE),1,'slot','master','',''),('B2','20',9.9,'pending_payment','pending',DATE_ADD(NOW(),INTERVAL 15 MINUTE),0,'','master','','')`)
	if e != nil {
		t.Fatal(e)
	}
	_, e = db.ExecCtx(ctx, `INSERT INTO askxuan_booking.consultation_order(order_no,user_id,master_code,consult_fee,status,payment_status,payment_no,payment_channel,valid_from,expires_at,valid_hours) VALUES('C1','20','other-master',9.9,'pending_payment','pending','','',NULL,NULL,24)`)
	if e != nil {
		t.Fatal(e)
	}
	_, e = db.ExecCtx(ctx, `INSERT INTO askxuan_ai.ai_report VALUES('AR0123456789abcdef0123456789abcdef','20',990,0,'ready'),('A2','20',990,1,'ready')`)
	if e != nil {
		t.Fatal(e)
	}
	if e = db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		return balance.Change(ctx, tx, "20", "test-fixture", "fixture", "test", 10000, 0)
	}); e != nil {
		t.Fatal(e)
	}
	// Both booking paths require a valid paid consultation for this user/master.
	for _, no := range []string{"B1", "B2"} {
		if _, e = s.Pay(ctx, "20", OrderRequest{OrderType: "booking", OrderNo: no, ExpectedCents: 990, Channel: "balance"}); !errors.Is(e, balance.ErrConflict) {
			t.Fatalf("booking without valid consultation: %s: %v", no, e)
		}
	}
	beforeBooking, e := balance.Read(ctx, db, "20")
	if e != nil || beforeBooking.Available != 10000 {
		t.Fatalf("rejected booking changed balance: %+v %v", beforeBooking, e)
	}
	_, e = db.ExecCtx(ctx, `INSERT INTO askxuan_booking.consultation_order(order_no,user_id,master_code,consult_fee,status,payment_status,payment_no,payment_channel,valid_from,expires_at,valid_hours) VALUES('C-valid','20','master',9.9,'active','success','fixture','balance',DATE_SUB(NOW(),INTERVAL 1 MINUTE),DATE_ADD(NOW(),INTERVAL 24 HOUR),24)`)
	if e != nil {
		t.Fatal(e)
	}
	for kind, nos := range map[string][]string{"booking": {"B1", "B2"}, "consultation": {"C1"}, "diy_order": {"D1"}, "ai_report": {"AR0123456789abcdef0123456789abcdef"}} {
		for _, no := range nos {
			if _, e = s.Pay(ctx, "20", OrderRequest{OrderType: kind, OrderNo: no, ExpectedCents: 990, Channel: "balance"}); e != nil {
				t.Fatal(kind, e)
			}
		}
	}
	// A repeated report checkout must preserve the full identifier and debit once.
	longReport := OrderRequest{OrderType: "ai_report", OrderNo: "AR0123456789abcdef0123456789abcdef", ExpectedCents: 990, Channel: "balance"}
	beforeRepeat, e := balance.Read(ctx, db, "20")
	if e != nil {
		t.Fatal(e)
	}
	reportPayment, e := s.Pay(ctx, "20", longReport)
	if e != nil || reportPayment.OrderNo != longReport.OrderNo {
		t.Fatalf("report identifier lost: %+v %v", reportPayment, e)
	}
	afterRepeat, e := balance.Read(ctx, db, "20")
	if e != nil || beforeRepeat.Available != afterRepeat.Available {
		t.Fatal("report checkout retry debited twice", e)
	}
	if _, e = s.Pay(ctx, "20", OrderRequest{OrderType: "ai_report", OrderNo: "A2", ExpectedCents: 990, Channel: "balance"}); e == nil {
		t.Fatal("points-unlocked report charged again")
	}
	t.Run("provider eligibility before pending charge", func(t *testing.T) { checkProviderPendingPayment(t, db) })
	var sum int64
	db.QueryRowCtx(ctx, &sum, `SELECT SUM(available_delta) FROM wallet_ledger WHERE user_id='10'`)
	if sum != a.Available {
		t.Fatalf("ledger/account mismatch %d %+v", sum, a)
	}
}

type fixtureGateway struct{}

func (fixtureGateway) Checkout(context.Context, paychannel.Order) (string, error) {
	return "", paychannel.ErrPending
}
func (fixtureGateway) Query(context.Context, string) (paychannel.Result, error) {
	return paychannel.Result{}, paychannel.ErrPending
}
func (fixtureGateway) Verify(*http.Request) (paychannel.Result, error) {
	return paychannel.Result{}, paychannel.ErrVerification
}
func (fixtureGateway) Refund(context.Context, paychannel.Refund) (string, error) {
	return "success", nil
}
