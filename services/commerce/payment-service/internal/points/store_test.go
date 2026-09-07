package points

import (
	"context"
	"errors"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestEarned(t *testing.T) {
	for _, c := range []struct{ cents, want int64 }{{-1, 0}, {0, 0}, {5800, 0}, {9999, 0}, {10000, 1}, {19999, 1}, {20000, 2}, {1234567, 123}} {
		if got := Earned(c.cents); got != c.want {
			t.Fatalf("%d cents: got %d want %d", c.cents, got, c.want)
		}
	}
}
func TestInputBounds(t *testing.T) {
	p := Product{Name: "礼品", PointsPrice: 1, Stock: 0, Status: "draft"}
	if !ValidProduct(p) {
		t.Fatal("valid product rejected")
	}
	p.PointsPrice = 0
	if ValidProduct(p) {
		t.Fatal("zero price accepted")
	}
	p.PointsPrice = 1
	p.Image = "javascript:alert(1)"
	if ValidProduct(p) {
		t.Fatal("unsafe image URL accepted")
	}
	if ValidRedeem(RedeemRequest{ProductID: 1, Quantity: 100}) {
		t.Fatal("invalid request accepted")
	}
}

// Explicit opt-in DSN MUST use an isolated, empty test database.
func TestMySQLPointsTransactions(t *testing.T) {
	dsn := os.Getenv("POINTS_TEST_DSN")
	if dsn == "" {
		t.Skip("POINTS_TEST_DSN not set")
	}
	ctx := context.Background()
	db := sqlx.NewMysql(dsn)
	s := Store{DB: db}
	schema, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "scripts", "db", "20260907_points_mall.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for pass := 0; pass < 2; pass++ {
		for _, stmt := range strings.Split(string(schema), ";") {
			if strings.TrimSpace(stmt) == "" {
				continue
			}
			if _, err := db.ExecCtx(ctx, stmt); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS payment(id BIGINT AUTO_INCREMENT PRIMARY KEY,user_id VARCHAR(64),payment_no VARCHAR(96),amount DECIMAL(12,2),status VARCHAR(16),idempotency_key VARCHAR(160),order_type VARCHAR(32) DEFAULT '',order_no VARCHAR(96) DEFAULT '',channel VARCHAR(16) DEFAULT '',trade_no VARCHAR(96) DEFAULT '',create_time DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`CREATE TABLE IF NOT EXISTS refund(id BIGINT AUTO_INCREMENT PRIMARY KEY,payment_id BIGINT,amount DECIMAL(12,2),status VARCHAR(16),refund_no VARCHAR(96) DEFAULT '',reason VARCHAR(255) DEFAULT '',create_time DATETIME DEFAULT CURRENT_TIMESTAMP)`,
		`INSERT INTO payment(id,user_id,payment_no,amount,status) VALUES(9001,'9001','PAY-test',299.99,'success'),(9002,'9002','PAY-test-2',1000,'success')`,
	} {
		if _, err := db.ExecCtx(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	award := func(id int64) error {
		return db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
			var n int64
			if err := tx.QueryRowCtx(ctx, &n, `SELECT id FROM payment WHERE id=? FOR UPDATE`, id); err != nil {
				return err
			}
			return Award(ctx, tx, id)
		})
	}
	if err := award(9001); err != nil {
		t.Fatal(err)
	}
	if err := award(9001); err != nil {
		t.Fatal(err)
	}
	if err := award(9002); err != nil {
		t.Fatal(err)
	}
	a, err := s.Account(ctx, "9001")
	if err != nil || a.Balance != 2 {
		t.Fatalf("duplicate award: %+v %v", a, err)
	}
	p, err := s.SaveProduct(ctx, Product{Name: "独立礼品", Description: "测试", PointsPrice: 2, Stock: 1, Status: "on_sale"})
	if err != nil {
		t.Fatal(err)
	}
	req := RedeemRequest{ProductID: p.ID, Quantity: 1, ExpectedPrice: 2, RequestKey: "same-request-00000001", Receiver: "测试", Mobile: "13800000000", Address: "测试地址"}
	var wg sync.WaitGroup
	var failures atomic.Int64
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Redeem(ctx, "9001", req); err != nil {
				failures.Add(1)
				t.Log(err)
			}
		}()
	}
	wg.Wait()
	if failures.Load() != 0 {
		t.Fatal("concurrent retries failed")
	}
	a, _ = s.Account(ctx, "9001")
	os1, err := s.Orders(ctx, "9001", false, 1)
	if err != nil || len(os1) != 1 || a.Balance != 0 {
		t.Fatalf("duplicate debit: %+v %+v %v", a, os1, err)
	}
	req.Quantity = 2
	if _, err = s.Redeem(ctx, "9001", req); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed payload accepted: %v", err)
	}
	req.Quantity = 1
	if _, err = s.SaveProduct(ctx, p); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale stock update accepted: %v", err)
	}
	if err = s.Transition(ctx, "9002", os1[0].ID, "cancel", "", "", false); !errors.Is(err, ErrInvalid) {
		t.Fatal("cross-user cancellation accepted")
	}
	// Partial refund crosses a threshold: 299.99 -> 199.99 means claw back one, even after spending.
	if _, err = db.ExecCtx(ctx, `INSERT INTO refund(id,payment_id,amount,status) VALUES(9001,9001,100,'success')`); err != nil {
		t.Fatal(err)
	}
	reverse := func() error {
		return db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error { return Reverse(ctx, tx, 9001, 9001) })
	}
	if err = reverse(); err != nil {
		t.Fatal(err)
	}
	if err = reverse(); err != nil {
		t.Fatal(err)
	}
	a, _ = s.Account(ctx, "9001")
	if a.Balance != -1 {
		t.Fatalf("refund debt: %+v", a)
	}
	if err = s.Transition(ctx, "9001", os1[0].ID, "cancel", "", "", false); err != nil {
		t.Fatal(err)
	}
	if err = s.Transition(ctx, "9001", os1[0].ID, "cancel", "", "", false); err != nil {
		t.Fatal(err)
	}
	a, _ = s.Account(ctx, "9001")
	if a.Balance != 1 {
		t.Fatalf("duplicate cancellation credit: %+v", a)
	}
	rows, _ := s.Products(ctx, true, 1)
	if rows[0].Stock != 1 {
		t.Fatal("stock not restored exactly once")
	}
	// Last item can only be redeemed by one of two concurrent funded requests.
	p2, err := s.SaveProduct(ctx, Product{Name: "限量礼品", PointsPrice: 1, Stock: 1, Status: "on_sale"})
	if err != nil {
		t.Fatal(err)
	}
	var successes atomic.Int64
	for _, user := range []string{"9001", "9002"} {
		wg.Add(1)
		go func(user string) {
			defer wg.Done()
			r := req
			r.ProductID = p2.ID
			r.ExpectedPrice = 1
			r.RequestKey = "last-stock-request-" + user
			if _, err := s.Redeem(ctx, user, r); err == nil {
				successes.Add(1)
			}
		}(user)
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("last stock successes=%d", successes.Load())
	}
	report, err := s.Report(ctx)
	if err != nil || report.PointsSpent != 1 || report.PendingCount != 1 {
		t.Fatalf("independent report: %+v %v", report, err)
	}
	// Transaction rollback leaves neither a balance delta nor a ledger record.
	err = db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		if e := Change(ctx, tx, "9002", "rollback-test", "earn", "test", 77); e != nil {
			return e
		}
		return ErrConflict
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	entries, err := s.Entries(ctx, "9002", 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.ReferenceNo == "test" {
			t.Fatal("rolled-back ledger persisted")
		}
	}
}

func TestMySQLFulfillment(t *testing.T) {
	dsn := os.Getenv("POINTS_TEST_DSN")
	if dsn == "" {
		t.Skip("POINTS_TEST_DSN not set")
	}
	ctx := context.Background()
	s := Store{DB: sqlx.NewMysql(dsn)}
	p, err := s.SaveProduct(ctx, Product{Name: "履约测试", PointsPrice: 2, Stock: 2, Status: "on_sale"})
	if err != nil {
		t.Fatal(err)
	}
	r := RedeemRequest{ProductID: p.ID, Quantity: 1, ExpectedPrice: 2, RequestKey: "fulfillment-test-00001", Receiver: "测试", Mobile: "13800000000", Address: "测试"}
	if _, err = s.Redeem(ctx, "unfunded-test-user", r); !errors.Is(err, ErrBalance) {
		t.Fatalf("insufficient balance: %v", err)
	}
	order, err := s.Redeem(ctx, "9002", r)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Transition(ctx, "9002", order.ID, "complete", "", "", false); !errors.Is(err, ErrConflict) {
		t.Fatalf("complete before shipping: %v", err)
	}
	if err = s.Transition(ctx, "9002", order.ID, "ship", "快递", "SF-test", false); !errors.Is(err, ErrInvalid) {
		t.Fatalf("customer shipped: %v", err)
	}
	if err = s.Transition(ctx, "admin", order.ID, "ship", "快递", "SF-test", true); err != nil {
		t.Fatal(err)
	}
	if err = s.Transition(ctx, "admin", order.ID, "ship", "快递", "SF-test", true); err != nil {
		t.Fatal(err)
	}
	if err = s.Transition(ctx, "admin", order.ID, "ship", "快递", "changed", true); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed shipment accepted: %v", err)
	}
	if err = s.Transition(ctx, "9002", order.ID, "cancel", "", "", false); !errors.Is(err, ErrConflict) {
		t.Fatalf("cancel after shipment: %v", err)
	}
	if err = s.Transition(ctx, "9002", order.ID, "complete", "", "", false); err != nil {
		t.Fatal(err)
	}
	rows, err := s.Orders(ctx, "9002", false, 1)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0].ID != order.ID || rows[0].Status != "completed" || rows[0].TrackingNo != "SF-test" {
		t.Fatalf("fulfillment result: %+v", rows)
	}
}
