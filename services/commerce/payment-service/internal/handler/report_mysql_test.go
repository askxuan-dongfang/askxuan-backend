package handler

import (
	"context"
	"errors"
	"github.com/askxuan/payment-service/internal/balance"
	"github.com/askxuan/payment-service/internal/cashier"
	"github.com/askxuan/payment-service/internal/model"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"sync"
	"testing"
)

func reportPaymentDB(t *testing.T) sqlx.SqlConn {
	t.Helper()
	dsn := os.Getenv("REPORT_PAYMENT_TEST_DSN")
	if dsn == "" {
		t.Skip("isolated REPORT_PAYMENT_TEST_DSN not set")
	}
	return sqlx.NewMysql(dsn)
}
func TestMySQLReportPurchase(t *testing.T) {
	db := reportPaymentDB(t)
	ctx := context.Background()
	var no string
	if e := db.QueryRowCtx(ctx, &no, `SELECT report_no FROM askxuan_ai.ai_report WHERE user_id='9501' AND request_key='mysql-report-case'`); e != nil {
		t.Fatal(e)
	}
	s := cashier.Store{DB: db, Enabled: true, Mock: true}
	req := cashier.OrderRequest{OrderType: "ai_report", OrderNo: no, ExpectedCents: 990, Channel: "balance"}
	if _, e := s.Pay(ctx, "9502", req); e == nil {
		t.Fatal("other user bought report")
	}
	bad := req
	bad.ExpectedCents = 1
	if _, e := s.Pay(ctx, "9501", bad); !errors.Is(e, balance.ErrConflict) {
		t.Fatal("tampered amount accepted", e)
	}
	for _, channel := range []string{"mock", "demo_balance", "wechat"} {
		bad = req
		bad.Channel = channel
		if _, e := s.Pay(ctx, "9501", bad); !errors.Is(e, balance.ErrUnavailable) {
			t.Fatal("AI accepted non-wallet channel", channel, e)
		}
	}
	if _, e := s.Pay(ctx, "9501", req); !errors.Is(e, balance.ErrFunds) {
		t.Fatal("insufficient cash did not roll back", e)
	}
	var payments int64
	_ = db.QueryRowCtx(ctx, &payments, `SELECT COUNT(*) FROM payment WHERE order_type='ai_report' AND order_no=?`, no)
	if payments != 0 {
		t.Fatal("insufficient wallet created payment")
	}
	if e := db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		return balance.Change(ctx, tx, "9501", "fixture:report-fund", "fixture", "fixture", 10000, 0)
	}); e != nil {
		t.Fatal(e)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, e := s.Pay(ctx, "9501", req); errs <- e }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	a, e := balance.Read(ctx, db, "9501")
	if e != nil || a.Available != 9010 {
		t.Fatal("duplicate debit", a, e)
	}
	var entries int64
	_ = db.QueryRowCtx(ctx, &entries, `SELECT COUNT(*) FROM wallet_ledger WHERE user_id='9501' AND kind='payment'`)
	if entries != 1 {
		t.Fatal("cash ledger duplication")
	}
	_ = db.QueryRowCtx(ctx, &entries, `SELECT COUNT(*) FROM points_ledger WHERE user_id='9501' AND event_key LIKE 'ai-report:%'`)
	if entries != 0 {
		t.Fatal("AI spent points")
	}
	for _, demo := range []bool{false, true} {
		s.DemoEnabled = demo
		q, e := s.Quote(ctx, "9501", req)
		if e != nil || q.Payment == nil || q.Payment.Channel != "balance" || q.Cents != 990 || q.MockEnabled {
			t.Fatal("paid report was requoted", q, e)
		}
		if _, e := s.Pay(ctx, "9501", req); e != nil {
			t.Fatal("historical debit retry after mode change", e)
		}
	}

}
func TestMySQLReportRefund(t *testing.T) {
	db := reportPaymentDB(t)
	ctx := context.Background()
	var id int64
	if e := db.QueryRowCtx(ctx, &id, `SELECT p.id FROM payment p JOIN askxuan_ai.ai_report r ON r.report_no=p.order_no WHERE p.order_type='ai_report' AND r.user_id='9501' AND r.request_key='mysql-report-case'`); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		if _, e := model.AtomicMockRefund(ctx, db, id, 9.9, "fixture refund"); e != nil {
			t.Fatal(e)
		}
	}
	a, e := balance.Read(ctx, db, "9501")
	if e != nil || a.Available != 10000 {
		t.Fatal("refund duplicated or lost", a, e)
	}
	if e = db.QueryRowCtx(ctx, &id, `SELECT id FROM payment WHERE order_type='ai_report' AND order_no='ARfixture-demo'`); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		if _, e = model.AtomicMockRefund(ctx, db, id, 9.9, "demo fixture refund"); e != nil {
			t.Fatal(e)
		}
	}
	demo, e := balance.ReadDemo(ctx, db, "9503")
	if e != nil || demo.Available != 2000 {
		t.Fatal("demo refund duplicated or lost", demo, e)
	}
}

func TestMySQLReportDemoPurchase(t *testing.T) {
	db := reportPaymentDB(t)
	ctx := context.Background()
	s := cashier.Store{DB: db, DemoEnabled: true, Mock: true}
	req := cashier.OrderRequest{OrderType: "ai_report", OrderNo: "ARfixture-demo", ExpectedCents: 990, Channel: "demo_balance"}
	q, e := s.Quote(ctx, "9503", req)
	if e != nil || q.BalanceChannel != "demo_balance" || q.Mode != "demo" || !q.BalanceEnabled || q.MockEnabled {
		t.Fatal("demo quote", q, e)
	}
	for _, channel := range []string{"mock", "balance"} {
		bad := req
		bad.Channel = channel
		if _, e = s.Pay(ctx, "9503", bad); !errors.Is(e, balance.ErrUnavailable) {
			t.Fatal("invalid demo channel", channel, e)
		}
	}
	if _, e = s.Pay(ctx, "9503", req); !errors.Is(e, balance.ErrFunds) {
		t.Fatal("demo insufficient funds", e)
	}
	e = db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		return balance.ChangeDemo(ctx, tx, "9503", "fixture:demo-fund", "fixture", "fixture", 2000, 0)
	})
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		if _, e = s.Pay(ctx, "9503", req); e != nil {
			t.Fatal(e)
		}
	}
	a, e := balance.ReadDemo(ctx, db, "9503")
	if e != nil || a.Available != 1010 {
		t.Fatal("duplicate demo debit", a, e)
	}
	real, e := balance.Read(ctx, db, "9503")
	if e != nil || real.Available != 0 {
		t.Fatal("demo moved real funds", real, e)
	}
	s.DemoEnabled = false
	s.Enabled = true
	q, e = s.Quote(ctx, "9503", req)
	if e != nil || q.Payment == nil || q.Payment.Channel != "demo_balance" || q.Cents != 990 {
		t.Fatal("historical demo lost", q, e)
	}
	if _, e = s.Pay(ctx, "9503", req); e != nil {
		t.Fatal("demo retry after mode change", e)
	}
}

func TestMySQLReportExistingPurchaseMatrix(t *testing.T) {
	db := reportPaymentDB(t)
	ctx := context.Background()
	s := cashier.Store{DB: db, Enabled: true, Mock: true}
	for _, tc := range []struct {
		no      string
		cents   int64
		channel string
		valid   bool
	}{
		{"ARmatrix-0", 800, "balance", true}, {"ARmatrix-1", 800, "balance", false},
		{"ARmatrix-2", 990, "balance", false}, {"ARmatrix-3", 990, "balance", false}, {"ARmatrix-4", 990, "balance", false},
		{"ARmatrix-5", 990, "balance", true}, {"ARmatrix-6", 990, "mock", false}, {"ARmatrix-7", 990, "demo_balance", false},
		{"ARmatrix-8", 990, "demo_balance", true}, {"ARmatrix-9", 990, "wechat", true},
	} {
		t.Run(tc.no, func(t *testing.T) {
			req := cashier.OrderRequest{OrderType: "ai_report", OrderNo: tc.no, ExpectedCents: tc.cents, Channel: tc.channel}
			q, e := s.GrowthQuote(ctx, "matrix-owner", req)
			if !tc.valid {
				if !errors.Is(e, balance.ErrConflict) {
					t.Fatal("invalid purchase accepted", q, e)
				}
				return
			}
			if e != nil || q.Payment == nil || q.Cents != tc.cents || q.Pricing != nil {
				t.Fatal("old paid amount repriced", q, e)
			}
			if tc.channel == "balance" || tc.channel == "demo_balance" {
				if _, e = s.Pay(ctx, "matrix-owner", req); e != nil {
					t.Fatal("old valid purchase retry failed", e)
				}
			}
		})
	}
}
