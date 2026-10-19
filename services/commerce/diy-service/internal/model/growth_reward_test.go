package model

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Uses the full disposable schema from scripts/test-growth.sh. No developer or
// production DSN is accepted and all fixture IDs are unique to this invocation.
func TestCreatorNetGrowthMySQL(t *testing.T) {
	dsn := os.Getenv("GROWTH_TEST_DSN")
	if dsn == "" {
		t.Skip("disposable growth schema required")
	}
	if os.Getenv("GROWTH_TEST_ISOLATED") != "1" || !strings.Contains(dsn, "tcp(127.0.0.1:") || !strings.Contains(dsn, "/askxuan_payment?") {
		t.Fatal("disposable localhost schema only")
	}
	db := sqlx.NewMysql(strings.Replace(dsn, "/askxuan_payment?", "/askxuan_diy?", 1))
	ctx := context.Background()
	logx.Disable()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := db.ExecCtx(ctx, q, args...); e != nil {
			t.Fatal(e)
		}
	}
	check := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	no := fmt.Sprintf("GN%d", time.Now().UnixNano())
	creator := "artist-" + no
	exec("INSERT INTO diy_order(order_no,user_id,material_fee,total_fee,status,payment_status,update_time) VALUES(?,'buyer',100,100,'completed','success',DATE_SUB(NOW(),INTERVAL 8 DAY))", no)
	var orderID int64
	check(db.QueryRowCtx(ctx, &orderID, "SELECT id FROM diy_order WHERE order_no=?", no))
	exec("UPDATE creator_reward_policy SET enabled=1,kind='cash',rate_bps=500,cap_cents=2000,wait_days=7")
	check(db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		return SnapshotReward(ctx, tx, &DiyOrder{Id: orderID, OrderNo: no, DesignId: 9, UserId: "buyer", CreatorId: creator, MaterialFee: 100})
	}))
	exec("INSERT INTO askxuan_payment.payment(payment_no,user_id,order_type,order_no,amount,channel,status) VALUES(?,'buyer','diy_order',?,92,'balance','success')", no, no)
	exec("INSERT INTO askxuan_payment.growth_quote(quote_id,user_id,order_type,order_no,root_checkout_id,rule_version,level,status,selected_kind,gross_cents,discount_cents,payable_cents,growth_base_cents,goods_net_cents,business_month,snapshot_json,expires_at,created_at) VALUES(?,'buyer','diy_order',?,?,'growth-v1',5,'paid','level',10000,800,9200,9200,9200,'2026-10','{}',0,1)", no, no, no)
	var rewardID int64
	check(db.QueryRowCtx(ctx, &rewardID, "SELECT id FROM creator_reward WHERE order_id=?", orderID))
	// Later edits to global policy cannot change the order's 5% / 20 yuan snapshot.
	exec("UPDATE creator_reward_policy SET rate_bps=1000,cap_cents=10000")
	check(processReward(ctx, db, rewardID))
	check(processReward(ctx, db, rewardID))
	var r struct {
		Base   int64  `db:"base_cents"`
		Amount int64  `db:"amount_cents"`
		Status string `db:"status"`
	}
	check(db.QueryRowCtx(ctx, &r, "SELECT base_cents,amount_cents,status FROM creator_reward WHERE id=?", rewardID))
	if r.Base != 9200 || r.Amount != 460 || r.Status != "settled" {
		t.Fatalf("net material policy snapshot: %+v", r)
	}
	var available int64
	check(db.QueryRowCtx(ctx, &available, "SELECT available_cents FROM creator_cash_account WHERE user_id=?", creator))
	if available != 460 {
		t.Fatalf("duplicate award: %d", available)
	}
	exec("INSERT INTO askxuan_payment.refund(refund_no,payment_id,amount,status) SELECT ?,id,92,'success' FROM askxuan_payment.payment WHERE payment_no=?", no, no)
	check(processReward(ctx, db, rewardID))
	check(processReward(ctx, db, rewardID))
	check(db.QueryRowCtx(ctx, &available, "SELECT available_cents FROM creator_cash_account WHERE user_id=?", creator))
	if available != 0 {
		t.Fatalf("refund reversal: %d", available)
	}
}
