package model

import (
	"context"
	"fmt"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestRewardAmountAndPolicy(t *testing.T) {
	for _, v := range []struct{ base, want int64 }{{10000, 500}, {50000, 2000}, {1, 0}, {199, 10}, {-1, 0}} {
		if got := RewardAmount(v.base, 500, 2000); got != v.want {
			t.Fatalf("%d: %d != %d", v.base, got, v.want)
		}
	}
	p := RewardPolicy{Kind: "cash", RateBps: 500, Cap: 2000, WaitDays: 7, MinWithdraw: 100}
	if p.Validate() != nil {
		t.Fatal("valid policy")
	}
	p.WaitDays = 0
	if p.Validate() == nil {
		t.Fatal("no waiting period")
	}
}
func TestPayoutEncryption(t *testing.T) {
	key := strings.Repeat("k", 32)
	v, e := SealPayout(key, "1", "6222000012345678")
	if e != nil || strings.Contains(v, "6222") {
		t.Fatal(e)
	}
	if out, e := OpenPayout(key, "1", v); e != nil || out != "6222000012345678" {
		t.Fatal(e)
	}
	if _, e = OpenPayout(key, "2", v); e == nil {
		t.Fatal("cross-user decrypt")
	}
}
func TestCreatorRewardsMySQL(t *testing.T) {
	dsn := os.Getenv("CREATOR_TEST_DSN")
	if dsn == "" {
		t.Skip("isolated MySQL required")
	}
	if !strings.Contains(dsn, "127.0.0.1:33379") || !strings.Contains(dsn, "/askxuan_diy") {
		t.Fatal("only dedicated local test database permitted")
	}
	db := sqlx.NewMysql(dsn)
	ctx := context.Background()
	key := strings.Repeat("k", 32)
	exec := func(q string, args ...interface{}) {
		t.Helper()
		if _, e := db.ExecCtx(ctx, q, args...); e != nil {
			t.Fatal(e)
		}
	}
	exec("CREATE TABLE IF NOT EXISTS askxuan_payment.growth_quote(quote_id CHAR(32) PRIMARY KEY,order_type VARCHAR(32),order_no VARCHAR(64),status VARCHAR(24),goods_net_cents BIGINT,created_at BIGINT)")
	exec("DELETE FROM askxuan_payment.growth_quote")
	for _, table := range []string{"creator_cash_ledger", "creator_cash_account", "creator_withdrawal", "creator_reward_audit", "creator_reward_delivery", "creator_reward", "diy_order", "askxuan_payment.refund", "askxuan_payment.payment", "askxuan_payment.wallet_ledger", "askxuan_payment.wallet_account", "askxuan_marketing.coupon_record", "askxuan_marketing.coupon"} {
		exec("DELETE FROM " + table)
	}
	exec("UPDATE creator_reward_policy SET enabled=1,kind='cash',rate_bps=500,cap_cents=2000,wait_days=7,min_withdraw_cents=100,coupon_id=0,revision=1")
	create := func(id int64, channel, kind string, days int) {
		t.Helper()
		no := fmt.Sprintf("TEST%d", id)
		exec("INSERT INTO diy_order VALUES(?,?,?,'completed','success',DATE_SUB(NOW(),INTERVAL ? DAY))", id, no, "buyer", days)
		exec("INSERT INTO askxuan_payment.payment VALUES(?,?,'diy_order','buyer','success',?)", id, no, channel)
		exec("UPDATE creator_reward_policy SET kind=?", kind)
		e := db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
			return SnapshotReward(ctx, tx, &DiyOrder{Id: id, OrderNo: no, DesignId: 9, UserId: "buyer", CreatorId: "artist", MaterialFee: 100, DesignSnapshot: `{"rewardPublicationId":1,"rewardDesignId":9}`})
		})
		if e != nil {
			t.Fatal(e)
		}
	}
	balance := func(want, held, paid int64) {
		t.Helper()
		var a CreatorAccount
		if e := db.QueryRowCtx(ctx, &a, "SELECT available_cents,held_cents,paid_cents FROM creator_cash_account WHERE user_id='artist'"); e != nil {
			t.Fatal(e)
		}
		if a.Available != want || a.Held != held || a.Paid != paid {
			t.Fatalf("account %+v expected %d/%d/%d", a, want, held, paid)
		}
	}
	rewardID := func(order int64) int64 {
		var id int64
		if e := db.QueryRowCtx(ctx, &id, "SELECT id FROM creator_reward WHERE order_id=?", order); e != nil {
			t.Fatal(e)
		}
		return id
	}
	process := func(order int64) {
		t.Helper()
		if e := processReward(ctx, db, rewardID(order)); e != nil {
			t.Fatal(e)
		}
	}
	status := func(order int64, want string) {
		t.Helper()
		var s string
		if e := db.QueryRowCtx(ctx, &s, "SELECT status FROM creator_reward WHERE order_id=?", order); e != nil || s != want {
			t.Fatalf("order %d status %s want %s: %v", order, s, want, e)
		}
	}
	create(1, "wechat", "cash", 8)
	process(1)
	process(1)
	balance(500, 0, 0)
	create(2, "mock", "cash", 8)
	process(2)
	status(2, "excluded")
	balance(500, 0, 0)
	create(3, "wechat", "cash", 6)
	process(3)
	status(3, "pending")
	balance(500, 0, 0)
	exec("INSERT INTO askxuan_payment.refund VALUES(1,1,'success')")
	process(1)
	process(1)
	balance(0, 0, 0)
	create(4, "alipay", "cash", 8)
	process(4)
	req := CreatorWithdrawal{Amount: 400, Key: "test-request-key-0001", Holder: "测试用户", Bank: "测试银行", Account: "6222000000000000"}
	id, e := ApplyCreatorWithdrawal(ctx, db, "artist", req, key)
	if e != nil {
		t.Fatal(e)
	}
	id2, e := ApplyCreatorWithdrawal(ctx, db, "artist", req, key)
	if e != nil || id != id2 {
		t.Fatal("idempotency", e)
	}
	balance(100, 400, 0)
	req.Key = "test-request-key-0002"
	if _, e = ApplyCreatorWithdrawal(ctx, db, "artist", req, key); e == nil {
		t.Fatal("overdraw accepted")
	}
	if e = ReviewCreatorWithdrawal(ctx, db, id, "paid", "BANK1", "confirmed", "admin"); e == nil {
		t.Fatal("paid before processing")
	}
	for _, a := range []string{"approve", "processing", "paid"} {
		if e = ReviewCreatorWithdrawal(ctx, db, id, a, "BANK1", "bank confirmed", "admin"); e != nil {
			t.Fatal(e)
		}
	}
	balance(100, 0, 400)
	if e = ReviewCreatorWithdrawal(ctx, db, id, "paid", "BANK1", "again", "admin"); e == nil {
		t.Fatal("duplicate paid")
	}
	exec("INSERT INTO askxuan_payment.refund VALUES(4,4,'success')")
	process(4)
	balance(-400, 0, 400)
	create(5, "wechat", "cash", 8)
	process(5)
	balance(100, 0, 400)
	// Concurrent applications must never reserve more than the available balance.
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			r := req
			r.Amount = 100
			r.Key = fmt.Sprintf("concurrent-request-%04d", i)
			if _, e := ApplyCreatorWithdrawal(ctx, db, "artist", r, key); e == nil {
				mu.Lock()
				success++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if success != 1 {
		t.Fatalf("concurrent successes %d", success)
	}
	balance(0, 100, 400)
	create(6, "balance", "balance", 8)
	process(6)
	process(6)
	var available int64
	if e = db.QueryRowCtx(ctx, &available, "SELECT available_cents FROM askxuan_payment.wallet_account WHERE user_id='artist'"); e != nil || available != 500 {
		t.Fatal("balance grant", e, available)
	}
	exec("INSERT INTO askxuan_marketing.coupon(id,coupon_no,name,type,value,min_amount,category_id,start_time,end_time,total_count,received_count,status) VALUES(10,'TEST','测试券','full_reduce',5,10,'',NOW(),DATE_ADD(NOW(),INTERVAL 30 DAY),10,0,'enabled')")
	exec("UPDATE creator_reward_policy SET coupon_id=10")
	create(7, "wechat", "coupon", 8)
	process(7)
	process(7)
	var count int
	if e = db.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM askxuan_marketing.coupon_record WHERE user_id='artist'"); e != nil || count != 1 {
		t.Fatal("coupon idempotency", e, count)
	}
	exec("INSERT INTO askxuan_payment.refund VALUES(7,7,'success')")
	process(7)
	var state string
	if e = db.QueryRowCtx(ctx, &state, "SELECT status FROM askxuan_marketing.coupon_record WHERE user_id='artist'"); e != nil || state != "expired" {
		t.Fatal("coupon reversal", e, state)
	}
}
