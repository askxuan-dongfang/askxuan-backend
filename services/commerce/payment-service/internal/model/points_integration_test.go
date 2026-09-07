package model

import (
	"context"
	"github.com/askxuan/payment-service/internal/points"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
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
