package model

import (
	"context"
	"github.com/askxuan/payment-service/internal/points"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"sync"
	"testing"
)

// Run after the points integration fixture in the same isolated test database.
func TestMySQLPaymentAndRefundPoints(t *testing.T) {
	dsn := os.Getenv("POINTS_TEST_DSN")
	if dsn == "" {
		t.Skip("POINTS_TEST_DSN not set")
	}
	ctx := context.Background()
	db := sqlx.NewMysql(dsn)
	p := &Payment{PaymentNo: "PAY-model-hook", UserId: "9200", OrderType: OrderTypeShopOrder, OrderNo: "SHOP-model-hook", Amount: 200, Channel: "mock"}
	paymentModel := NewPaymentModel(db)
	created, err := paymentModel.Insert(ctx, p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = paymentModel.UpdateStatus(ctx, created.Id, PaymentStatusSuccess, "MOCK-model"); err != nil {
		t.Fatal(err)
	}
	if _, err = paymentModel.UpdateStatus(ctx, created.Id, PaymentStatusSuccess, "MOCK-model"); err != nil {
		t.Fatal(err)
	}
	store := points.Store{DB: db}
	a, err := store.Account(ctx, "9200")
	if err != nil || a.Balance != 2 {
		t.Fatalf("payment did not award exactly once: %+v %v", a, err)
	}
	if _, err = paymentModel.UpdateStatus(ctx, created.Id, PaymentStatusRefunding, ""); err != nil {
		t.Fatal(err)
	}
	refunds := NewRefundModel(db)
	r, err := refunds.Insert(ctx, &Refund{PaymentId: created.Id, Amount: 100, Reason: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = refunds.UpdateStatus(ctx, r.Id, RefundStatusSuccess); err != nil {
		t.Fatal(err)
	}
	if _, err = refunds.UpdateStatus(ctx, r.Id, RefundStatusSuccess); err != nil {
		t.Fatal(err)
	}
	if _, err = paymentModel.UpdateStatus(ctx, created.Id, PaymentStatusRefunded, ""); err != nil {
		t.Fatal(err)
	}
	a, err = store.Account(ctx, "9200")
	if err != nil || a.Balance != 1 {
		t.Fatalf("refund did not reverse exactly once: %+v %v", a, err)
	}
	if _, err = paymentModel.UpdateStatus(ctx, created.Id, PaymentStatusSuccess, ""); err == nil {
		t.Fatal("refunded payment awarded twice")
	}
}

func TestMySQLAtomicCommerceRefund(t *testing.T) {
	dsn := os.Getenv("POINTS_TEST_DSN")
	if dsn == "" {
		t.Skip("requires isolated points fixture")
	}
	ctx := context.Background()
	db := sqlx.NewMysql(dsn)
	pm := NewPaymentModel(db)
	p, err := pm.Insert(ctx, &Payment{PaymentNo: "PAY-atomic-commerce", UserId: "9400", OrderType: OrderTypeShopOrder, OrderNo: "SHOP-atomic-commerce", Amount: 200, Channel: "mock"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pm.UpdateStatus(ctx, p.Id, PaymentStatusSuccess, "MOCK-atomic"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecCtx(ctx, `CREATE TRIGGER fail_commerce_refund BEFORE INSERT ON points_ledger FOR EACH ROW BEGIN IF NEW.kind='refund' THEN SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='injected rollback'; END IF; END`); err != nil {
		t.Fatal(err)
	}
	if _, err = AtomicMockRefund(ctx, db, p.Id, 200, "rollback"); err == nil {
		t.Fatal("injected failure not propagated")
	}
	var count int64
	if err = db.QueryRowCtx(ctx, &count, `SELECT COUNT(*) FROM refund WHERE payment_id=?`, p.Id); err != nil || count != 0 {
		t.Fatal("refund escaped rollback")
	}
	current, err := pm.FindByPaymentNo(ctx, p.PaymentNo)
	if err != nil || current.Status != PaymentStatusSuccess {
		t.Fatal("payment escaped rollback")
	}
	if _, err = db.ExecCtx(ctx, `DROP TRIGGER fail_commerce_refund`); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	ids := make(chan int64, 8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := AtomicMockRefund(ctx, db, p.Id, 200, "concurrent refund")
			if result != nil {
				ids <- result.Id
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var id int64
	for got := range ids {
		if id != 0 && id != got {
			t.Fatal("duplicate refund")
		}
		id = got
	}
	account, err := (points.Store{DB: db}).Account(ctx, "9400")
	if err != nil || account.Balance != 0 {
		t.Fatalf("incorrect balance: %+v %v", account, err)
	}
}
