package wallet

import (
	"context"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"strings"
	"testing"
)

func TestInvalidQueriesFailBeforeDatabase(t *testing.T) {
	for _, args := range []struct {
		user, mode, filter string
		page               int
	}{{"", "mock", "all", 1}, {"1", "bad", "all", 1}, {"1", "mock", "all", 0}, {"1", "mock", "bad", 1}} {
		if _, e := (Store{}).Read(context.Background(), args.user, args.mode, args.filter, args.page); e == nil {
			t.Fatal("invalid query accepted")
		}
	}
}
func TestMySQLWalletIsolationAndRefundAccounting(t *testing.T) {
	dsn := os.Getenv("WALLET_TEST_DSN")
	if dsn == "" {
		t.Skip("isolated WALLET_TEST_DSN not set")
	}
	if !strings.Contains(dsn, "/wallet_test") {
		t.Fatal("test requires isolated wallet_test database")
	}
	db := sqlx.NewMysql(dsn)
	ctx := context.Background()
	for _, q := range []string{
		`DROP TABLE IF EXISTS refund`, `DROP TABLE IF EXISTS payment`,
		`CREATE TABLE payment(id BIGINT PRIMARY KEY,payment_no VARCHAR(32),user_id VARCHAR(32),order_type VARCHAR(32),order_no VARCHAR(32),amount DECIMAL(10,2),channel VARCHAR(32),status VARCHAR(32),create_time DATETIME)`,
		`CREATE TABLE refund(id BIGINT PRIMARY KEY,payment_id BIGINT,refund_no VARCHAR(32),amount DECIMAL(10,2),status VARCHAR(32),reason VARCHAR(100),create_time DATETIME)`,
		`INSERT INTO payment VALUES(1,'PAY1','91001','booking','B1',100.01,'mock','success',NOW()),(2,'PAY2','91002','booking','B2',999.99,'mock','success',NOW()),(3,'PAY3','91001','booking','B3',12.34,'wechat','success',NOW()),(4,'PAY4','91001','booking','B4',8.00,'mock','failed',NOW())`,
		`INSERT INTO refund VALUES(1,1,'RF1',10.01,'success','部分退款',NOW()),(2,1,'RF2',20.02,'success','部分退款',NOW()),(3,1,'RF3',3.03,'processing','处理中',NOW()),(4,1,'RF4',4.04,'failed','失败',NOW()),(5,2,'RF5',88,'success','其他用户',NOW())`,
	} {
		if _, e := db.ExecCtx(ctx, q); e != nil {
			t.Fatal(e)
		}
	}
	s := Store{DB: db}
	page, e := s.Read(ctx, "91001", "mock", "all", 1)
	if e != nil {
		t.Fatal(e)
	}
	if page.Total != 2 || len(page.List) != 2 || page.Summary.PaidCents != 10001 || page.Summary.RefundedCents != 3003 || page.Summary.RefundingCents != 303 {
		t.Fatalf("wrong aggregation %+v", page)
	}
	if len(page.List[1].Refunds) != 4 {
		t.Fatal("refunds missing")
	}
	filtered, e := s.Read(ctx, "91001", "mock", "refunds", 1)
	if e != nil || filtered.Total != 1 || filtered.List[0].OrderNo != "B1" {
		t.Fatalf("refund filter leaked: %+v %v", filtered, e)
	}
	channel, e := s.Read(ctx, "91001", "channel", "all", 1)
	if e != nil || channel.Summary.PaidCents != 1234 || channel.Total != 1 {
		t.Fatalf("mode separation failed %+v %v", channel, e)
	}
	empty, e := s.Read(ctx, "91003", "mock", "all", 1)
	if e != nil || empty.Total != 0 || len(empty.List) != 0 || empty.Summary.PaidCents != 0 {
		t.Fatalf("account isolation failed %+v %v", empty, e)
	}
	next, e := s.Read(ctx, "91001", "mock", "all", 2)
	if e != nil || len(next.List) != 0 || next.Summary != page.Summary {
		t.Fatalf("pagination changed summary %+v %v", next, e)
	}
}
