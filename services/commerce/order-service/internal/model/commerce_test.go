package model

import (
	"context"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"sync"
	"testing"
)

func TestMySQLReturnFulfillment(t *testing.T) {
	dsn := os.Getenv("COMMERCE_TEST_DSN")
	if dsn == "" {
		t.Skip("requires isolated COMMERCE_TEST_DSN")
	}
	db := sqlx.NewMysql(dsn)
	ctx := context.Background()
	for _, ddl := range []string{
		`CREATE TABLE shop_order(id BIGINT PRIMARY KEY,user_id VARCHAR(64),status VARCHAR(32),pay_amount DECIMAL(10,2),update_time DATETIME)`,
		`CREATE TABLE return_order(id BIGINT PRIMARY KEY AUTO_INCREMENT,return_no VARCHAR(32) UNIQUE,order_id BIGINT,type VARCHAR(32),reason VARCHAR(255),status VARCHAR(32),refund_amount DECIMAL(10,2),create_time DATETIME,update_time DATETIME)`,
		`CREATE TABLE return_fulfillment(return_id BIGINT PRIMARY KEY,previous_status VARCHAR(32),carrier VARCHAR(80) DEFAULT '',tracking_no VARCHAR(100) DEFAULT '',review_note VARCHAR(500) DEFAULT '')`,
		`INSERT INTO shop_order VALUES(1,'customer-a','shipped',200,NOW()),(2,'customer-a','paid',100,NOW()),(3,'customer-a','pending_payment',100,NOW())`,
	} {
		if _, err := db.ExecCtx(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := CreateReturn(ctx, db, "customer-b", 1, "return", "wrong user"); err == nil {
		t.Fatal("cross-user return allowed")
	}
	if _, err := CreateReturn(ctx, db, "customer-a", 3, "return", "unpaid"); err == nil {
		t.Fatal("unpaid return allowed")
	}
	var wg sync.WaitGroup
	ids := make(chan int64, 8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := CreateReturn(ctx, db, "customer-a", 1, "return", "尺寸不合适")
			ids <- r.Id
			errs <- err
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	var id int64
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	for got := range ids {
		if id != 0 && got != id {
			t.Fatal("duplicate return created")
		}
		id = got
	}
	if err := MoveReturn(ctx, db, id, "", "receive", "", "", ""); err == nil {
		t.Fatal("received before approval")
	}
	if err := MoveReturn(ctx, db, id, "", "approve", "", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := MoveReturn(ctx, db, id, "customer-b", "ship", "顺丰", "SF1", ""); err == nil {
		t.Fatal("cross-user shipment allowed")
	}
	for i := 0; i < 2; i++ {
		if err := MoveReturn(ctx, db, id, "customer-a", "ship", "顺丰", "SF1", ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := MoveReturn(ctx, db, id, "customer-a", "ship", "顺丰", "CHANGED", ""); err == nil {
		t.Fatal("repeated shipment overwrote tracking")
	}
	for i := 0; i < 2; i++ {
		if err := MoveReturn(ctx, db, id, "", "receive", "", "", ""); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := ReturnDetails(ctx, db, 1)
	if err != nil || len(rows) != 1 || rows[0].TrackingNo != "SF1" || rows[0].Status != "return_received" {
		t.Fatalf("lost fulfillment: %+v %v", rows, err)
	}
	r, err := CreateReturn(ctx, db, "customer-a", 2, "return", "未发货退款")
	if err != nil {
		t.Fatal(err)
	}
	if err := MoveReturn(ctx, db, r.Id, "", "reject", "", "", "说明"); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := db.QueryRowCtx(ctx, &status, `SELECT status FROM shop_order WHERE id=2`); err != nil || status != "paid" {
		t.Fatalf("rejection did not restore order: %s %v", status, err)
	}
	r, err = CreateReturn(ctx, db, "customer-a", 2, "return", "再次申请")
	if err != nil {
		t.Fatal(err)
	}
	if err := MoveReturn(ctx, db, r.Id, "", "approve", "", "", ""); err != nil {
		t.Fatal(err)
	}
	rows, err = ReturnDetails(ctx, db, 2)
	if err != nil || rows[0].Status != "return_received" {
		t.Fatal("unshipped refund unnecessarily requires physical return")
	}
}
