package cashier

import (
	"context"
	"github.com/askxuan/payment-service/internal/balance"
	"github.com/askxuan/payment-service/internal/model"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"strings"
	"sync"
	"testing"
)

func checkDemoWallet(t *testing.T, db sqlx.SqlConn) {
	t.Helper()
	ctx := context.Background()
	migration, e := os.ReadFile("../../../../../scripts/db/20260928_demo_wallet.sql")
	if e != nil {
		t.Fatal(e)
	}
	for _, q := range strings.Split(strings.Split(string(migration), "CREATE TABLE IF NOT EXISTS askxuan_finance")[0], ";") {
		if strings.TrimSpace(q) != "" {
			if _, e = db.ExecCtx(ctx, q); e != nil {
				t.Fatal(e)
			}
		}
	}
	s := Store{DB: db, Mock: true, DemoEnabled: true}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, e := s.DemoRecharge(ctx, "demo-owner", "idempotency-demo-0001", 10000); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	a, e := balance.ReadDemo(ctx, db, "demo-owner")
	if e != nil || a.Available != 10000 {
		t.Fatalf("duplicate recharge: %+v %v", a, e)
	}
	if _, e = s.DemoRecharge(ctx, "demo-owner", "idempotency-demo-0001", 20000); e == nil {
		t.Fatal("changed request accepted")
	}
	if _, e = s.DemoRecharge(ctx, "demo-owner", "idempotency-demo-0002", 0); e == nil {
		t.Fatal("invalid amount accepted")
	}
	_, e = db.ExecCtx(ctx, `INSERT INTO askxuan_order.shop_order VALUES('DEMO-1','demo-owner',80,'pending_payment'),('DEMO-2','demo-owner',80,'pending_payment')`)
	if e != nil {
		t.Fatal(e)
	}
	req := OrderRequest{OrderType: "shop_order", OrderNo: "DEMO-1", Channel: "demo_balance", ExpectedCents: 8000}
	if _, e = s.Pay(ctx, "foreign", req); e == nil {
		t.Fatal("foreign order paid")
	}
	for _, channel := range []string{"balance", "mock"} {
		bad := req
		bad.Channel = channel
		if _, e = s.Pay(ctx, "demo-owner", bad); e == nil {
			t.Fatal("bypass accepted", channel)
		}
	}
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, e := s.Pay(ctx, "demo-owner", req); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	a, e = balance.ReadDemo(ctx, db, "demo-owner")
	if e != nil || a.Available != 2000 {
		t.Fatalf("duplicate debit: %+v %v", a, e)
	}
	bad := req
	bad.OrderNo = "DEMO-2"
	if _, e = s.Pay(ctx, "demo-owner", bad); e == nil {
		t.Fatal("insufficient funds accepted")
	}
	var count int64
	if e = db.QueryRowCtx(ctx, &count, `SELECT COUNT(*) FROM payment WHERE order_no='DEMO-2'`); e != nil || count != 0 {
		t.Fatal("failed debit left payment", count, e)
	}
	paid, e := model.NewPaymentModel(db).FindByOrder(ctx, "shop_order", "DEMO-1")
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, e := model.AtomicMockRefund(ctx, db, paid.Id, 80, "demo test"); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	a, e = balance.ReadDemo(ctx, db, "demo-owner")
	if e != nil || a.Available != 10000 {
		t.Fatalf("refund mismatch: %+v %v", a, e)
	}
	// Retry the previously rejected order after its funds have been restored.
	if _, e = s.Pay(ctx, "demo-owner", bad); e != nil {
		t.Fatal("insufficient-funds retry", e)
	}
	a, e = balance.ReadDemo(ctx, db, "demo-owner")
	if e != nil || a.Available != 2000 {
		t.Fatal("retry debit", a, e)
	}
	real, e := balance.Read(ctx, db, "demo-owner")
	if e != nil || real.Available != 0 {
		t.Fatal("demo touched real balance", real, e)
	}
	if e = db.QueryRowCtx(ctx, &count, `SELECT COUNT(*) FROM points_payment_award WHERE user_id='demo-owner'`); e != nil || count != 0 {
		t.Fatal("demo awarded points", count, e)
	}
	f, e := s.DemoRecharge(ctx, "demo-return", "demo-refund-request-01", 1000)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.demoRefund(ctx, "foreign", f.No, 1000); e == nil {
		t.Fatal("foreign recharge refunded")
	}
	for i := 0; i < 2; i++ {
		if _, e = s.demoRefund(ctx, "demo-return", f.No, 1000); e != nil {
			t.Fatal(e)
		}
	}
	a, e = balance.ReadDemo(ctx, db, "demo-return")
	if e != nil || a.Available != 0 {
		t.Fatal("recharge refund duplicated", a, e)
	}
	disabled := s
	disabled.DemoEnabled = false
	if _, e = disabled.DemoRecharge(ctx, "demo-owner", "disabled-request-001", 100); e == nil {
		t.Fatal("disabled demo credited")
	}
}
