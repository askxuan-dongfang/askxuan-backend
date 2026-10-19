package growth_test

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/askxuan/payment-service/internal/balance"
	"github.com/askxuan/payment-service/internal/cashier"
	"github.com/askxuan/payment-service/internal/growth"
	"github.com/askxuan/payment-service/internal/model"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// The harness loads the full schema into a disposable localhost MySQL. It never
// accepts a remote host or a production/developer database connection.
func TestGrowthMySQL(t *testing.T) {
	dsn := os.Getenv("GROWTH_TEST_DSN")
	if dsn == "" {
		t.Skip("isolated MySQL required")
	}
	if os.Getenv("GROWTH_TEST_ISOLATED") != "1" || !strings.Contains(dsn, "tcp(127.0.0.1:") || !strings.Contains(dsn, "/askxuan_payment?") {
		t.Fatal("disposable local schema required")
	}
	logx.Disable()
	ctx := context.Background()
	db := sqlx.NewMysql(dsn)
	now := time.Now()
	g := &growth.Store{DB: db, Now: func() time.Time { return now }}
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := db.ExecCtx(ctx, q, args...); e != nil {
			t.Fatalf("%s: %v", q, e)
		}
	}
	check := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	scalar := func(q string, args ...any) int64 {
		t.Helper()
		var n int64
		check(db.QueryRowCtx(ctx, &n, q, args...))
		return n
	}
	suffix := fmt.Sprint(now.UnixNano())
	user := "g" + suffix
	exec("UPDATE growth_settings SET enabled=1,discounts_enabled=1")
	exec("UPDATE growth_budget SET total_cents=10000000")
	t.Run("private notes idempotency and midnight", func(t *testing.T) {
		var wg sync.WaitGroup
		errs := make(chan error, 8)
		for range 8 {
			wg.Add(1)
			go func() { defer wg.Done(); errs <- g.SaveNote(ctx, user, "阅读并实践", "只对本人可见", false) }()
		}
		wg.Wait()
		close(errs)
		for e := range errs {
			check(e)
		}
		v, e := g.Overview(ctx, user)
		check(e)
		if v.Total != 2 {
			t.Fatalf("duplicate note: %d", v.Total)
		}
		check(g.DeleteNote(ctx, user, growth.BusinessDay(now)))
		v, e = g.Overview(ctx, user)
		check(e)
		if v.Total != 2 {
			t.Fatal("deleting a private note reversed growth")
		}
		now = now.Add(24 * time.Hour)
		check(g.SaveNote(ctx, user, "整理今日所学", "", false))
		v, e = g.Overview(ctx, user)
		check(e)
		if v.Total != 4 {
			t.Fatal("new business day not awarded")
		}
	})
	t.Run("two admins and lifetime gifts", func(t *testing.T) {
		raw, _ := json.Marshal(growth.Adjustment{User: user, Amount: 196})
		id, e := g.Propose(ctx, "admin-a", "adjustment", "补充测试账户成长值", raw)
		check(e)
		if g.Approve(ctx, "admin-a", id, true) == nil {
			t.Fatal("self approval allowed")
		}
		check(g.Approve(ctx, "admin-b", id, true))
		v, e := g.Overview(ctx, user)
		check(e)
		if v.Total != 200 || v.Level.Code != 2 || len(v.Coupons) != 1 || v.Coupons[0].Status != "unused" {
			t.Fatalf("level or gift %+v", v)
		}
	})
	t.Run("learning requires visible elapsed time", func(t *testing.T) {
		id, e := g.SaveContent(ctx, "admin-a", growth.Content{Title: "学习测试", Kind: "article", Body: strings.Repeat("学习内容", 50), Enabled: true})
		check(e)
		l, e := g.StartLearning(ctx, user, id)
		check(e)
		now = now.Add(10 * time.Second)
		l, e = g.LearningHeartbeat(ctx, user, l.ID, false, true)
		check(e)
		if l.Elapsed != 0 || l.Completed {
			t.Fatal("background time counted")
		}
		for range 6 {
			now = now.Add(10 * time.Second)
			l, e = g.LearningHeartbeat(ctx, user, l.ID, true, true)
			check(e)
		}
		if !l.Completed {
			t.Fatal("valid learning not completed")
		}
		v, e := g.Overview(ctx, user)
		check(e)
		if v.Total != 203 {
			t.Fatalf("learning total %d", v.Total)
		}
	})
	t.Run("activities verified by evidence", func(t *testing.T) {
		id, e := g.SaveActivity(ctx, "admin-a", growth.Activity{Title: "测试活动", Description: "测试活动说明", Starts: now.Unix() - 60, Ends: now.Unix() + 3600, Enabled: true})
		check(e)
		check(g.RegisterActivity(ctx, user, id))
		check(g.VerifyAttendance(ctx, "admin-a", user, "签到核验记录 001", id))
		check(g.VerifyAttendance(ctx, "admin-a", user, "重试核验记录", id))
		v, e := g.Overview(ctx, user)
		check(e)
		if v.Total != 223 {
			t.Fatalf("activity total %d", v.Total)
		}
	})
	t.Run("net payment snapshot wait and atomic full refund", func(t *testing.T) {
		no := "GSO" + suffix
		exec("INSERT INTO askxuan_order.shop_order(order_no,user_id,total_amount,pay_amount) VALUES(?,?,100,100)", no, user)
		id := scalar("SELECT id FROM askxuan_order.shop_order WHERE order_no=?", no)
		exec("INSERT INTO askxuan_order.shop_order_item(order_id,product_id,sku_id,product_name,price,quantity) VALUES(?,1,991019,'测试商品',100,1)", id)
		exec("INSERT INTO growth_item_policy(item_type,item_id,title,owner_type,owner_id,enabled,consent,cost_cents,minimum_contribution_cents,subsidy_cap_cents,revision,updated_at) VALUES('shop_sku','991019','测试商品','platform','',1,1,100,100,1000,1,?) ON DUPLICATE KEY UPDATE enabled=1,consent=1", now.Unix())
		check(db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
			return balance.Change(ctx, tx, user, "fund:"+suffix, "test", "test", 100000, 0)
		}))
		c := cashier.Store{DB: db, Enabled: true, Growth: g}
		req := cashier.OrderRequest{OrderType: "shop_order", OrderNo: no}
		q, e := c.GrowthQuote(ctx, user, req)
		check(e)
		if q.Pricing == nil {
			t.Fatal("missing pricing")
		}
		var offer growth.Offer
		for _, v := range q.Pricing.Snapshot.Offers {
			if v.Kind == "level" {
				offer = v
			}
		}
		if offer.Payable != 9800 || offer.Growth != 98 {
			t.Fatalf("bad level offer %+v", offer)
		}
		req.QuoteID = q.Pricing.ID
		req.OfferKind = "level"
		req.ExpectedCents = 9800
		req.Channel = "balance"
		_, e = c.Reserve(ctx, user, req)
		check(e)
		_, e = c.Reserve(ctx, user, req)
		check(e)
		paid, e := c.Pay(ctx, user, req)
		check(e)
		again, e := c.Pay(ctx, user, req)
		check(e)
		if paid.Id != again.Id || paid.Amount != 98 {
			t.Fatal("not one net payment")
		}
		check(g.ReconcilePayment(ctx, paid.Id))
		v, e := g.Overview(ctx, user)
		check(e)
		if v.Pending != 98 || v.Total != 223 {
			t.Fatalf("before completion %+v", v)
		}
		exec("UPDATE askxuan_order.shop_order SET status='completed',update_time=FROM_UNIXTIME(?) WHERE order_no=?", now.Unix(), no)
		check(g.ReconcilePayment(ctx, paid.Id))
		now = now.Add(7*24*time.Hour - time.Second)
		check(g.ReconcilePayment(ctx, paid.Id))
		v, e = g.Overview(ctx, user)
		check(e)
		if v.Total != 223 {
			t.Fatal("early growth")
		}
		now = now.Add(time.Second)
		check(g.ReconcilePayment(ctx, paid.Id))
		check(g.ReconcilePayment(ctx, paid.Id))
		v, e = g.Overview(ctx, user)
		check(e)
		if v.Total != 321 {
			t.Fatalf("mature total %d", v.Total)
		}
		amount, e := g.FullRefundAmount(ctx, paid.PaymentNo, "shop_order", no, 100)
		check(e)
		if amount != 98 {
			t.Fatal("full refund not net")
		}
		if _, e = g.FullRefundAmount(ctx, paid.PaymentNo, "shop_order", no, 99); e == nil {
			t.Fatal("arbitrary amount reinterpreted")
		}
		_, e = model.AtomicMockRefund(ctx, db, paid.Id, 98, "集成测试整单退款", g)
		check(e)
		_, e = model.AtomicMockRefund(ctx, db, paid.Id, 98, "重试", g)
		check(e)
		v, e = g.Overview(ctx, user)
		check(e)
		if v.Total != 223 || v.Pending != 0 {
			t.Fatalf("refund total %+v", v)
		}
		if scalar("SELECT spent_cents FROM growth_budget WHERE kind='level'") != 0 {
			t.Fatal("subsidy not released")
		}
		a, e := balance.Read(ctx, db, user)
		check(e)
		if a.Available != 100000 {
			t.Fatal("cash not restored")
		}
	})
	t.Run("read models support and ownership", func(t *testing.T) {
		_, e := g.Records(ctx, user, 1)
		check(e)
		_, e = g.Contents(ctx, false)
		check(e)
		_, e = g.Activities(ctx, user, false)
		check(e)
		_, e = g.Dashboard(ctx)
		check(e)
		check(g.RequestSupport(ctx, user, "support-"+suffix, "growth", "请帮助核对成长记录"))
		check(g.RequestSupport(ctx, user, "support-"+suffix, "growth", "重复请求"))
		list, e := g.Support(ctx, user, false)
		check(e)
		if len(list) != 1 {
			t.Fatal("support duplicate")
		}
		check(g.ReplySupport(ctx, "admin-b", list[0].ID, "核对完成", 0))
		notes, e := g.Notes(ctx, "other-"+suffix)
		check(e)
		if len(notes) != 0 {
			t.Fatal("private notes leaked")
		}
		_, e = g.ProviderPolicies(ctx, "temple", "T001")
		check(e)
		_, e = g.ProviderBills(ctx, "temple", "T001")
		check(e)
	})
	t.Run("gift reservation replacement payment and refund", func(t *testing.T) {
		u := "gift" + suffix
		raw, _ := json.Marshal(growth.Adjustment{User: u, Amount: 200})
		proposal, e := g.Propose(ctx, "admin-a", "adjustment", "礼券事务测试账户", raw)
		check(e)
		check(g.Approve(ctx, "admin-b", proposal, true))
		no := "GIFT" + suffix
		exec("INSERT INTO askxuan_order.shop_order(order_no,user_id,total_amount,pay_amount) VALUES(?,?,100,100)", no, u)
		id := scalar("SELECT id FROM askxuan_order.shop_order WHERE order_no=?", no)
		exec("INSERT INTO askxuan_order.shop_order_item(order_id,product_id,sku_id,product_name,price,quantity) VALUES(?,1,991019,'礼券测试',100,1)", id)
		check(db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
			return balance.Change(ctx, tx, u, "gift"+suffix, "test", "test", 10000, 0)
		}))
		c := cashier.Store{DB: db, Enabled: true, Growth: g}
		r := cashier.OrderRequest{OrderType: "shop_order", OrderNo: no, Channel: "balance"}
		q, e := c.GrowthQuote(ctx, u, r)
		check(e)
		if q.Pricing.Snapshot.Selected.Kind != "coupon" || q.Cents != 9500 {
			t.Fatal("best gift price", q)
		}
		r.QuoteID = q.Pricing.ID
		r.OfferKind = "coupon"
		r.CouponID = q.Pricing.CouponID
		r.ExpectedCents = 9500
		_, e = c.Reserve(ctx, u, r)
		check(e)
		// Reopening cashier must release the earlier reservation before quoting.
		q, e = c.GrowthQuote(ctx, u, cashier.OrderRequest{OrderType: r.OrderType, OrderNo: no})
		check(e)
		if q.Pricing.ID == r.QuoteID || q.Cents != 9500 {
			t.Fatal("old reservation retained")
		}
		r.QuoteID = q.Pricing.ID
		_, e = c.Reserve(ctx, u, r)
		check(e)
		paid, e := c.Pay(ctx, u, r)
		check(e)
		_, e = model.AtomicMockRefund(ctx, db, paid.Id, 95, "整单退款", g)
		check(e)
		if scalar("SELECT COUNT(*) FROM growth_coupon WHERE user_id=? AND status='unused'", u) != 1 {
			t.Fatal("valid gift not restored")
		}
		// A downgrade revokes it once; regaining the level never issues it again.
		event := scalar("SELECT id FROM growth_event WHERE user_id=? AND kind='adjustment'", u)
		revoke, _ := json.Marshal(map[string]any{"userId": u, "sourceEventId": event, "amount": 0})
		proposal, e = g.Propose(ctx, "admin-a", "adjustment", "撤回测试补偿来源", revoke)
		check(e)
		check(g.Approve(ctx, "admin-b", proposal, true))
		proposal, e = g.Propose(ctx, "admin-a", "adjustment", "重新补偿升级测试", raw)
		check(e)
		check(g.Approve(ctx, "admin-b", proposal, true))
		if scalar("SELECT COUNT(*) FROM growth_coupon WHERE user_id=? AND status='revoked'", u) != 1 {
			t.Fatal("gift reissued")
		}
	})
	t.Run("global budget contention and expiry while switched off", func(t *testing.T) {
		exec("UPDATE growth_budget SET total_cents=spent_cents+held_cents+200 WHERE kind='level'")
		c := cashier.Store{DB: db, Enabled: true, Growth: g}
		requests := make([]cashier.OrderRequest, 2)
		users := []string{"raceA" + suffix, "raceB" + suffix}
		for i, u := range users {
			exec("INSERT INTO growth_account(user_id,total,updated_at) VALUES(?,200,?)", u, now.Unix())
			no := fmt.Sprintf("R%d%s", i, suffix)
			exec("INSERT INTO askxuan_order.shop_order(order_no,user_id,total_amount,pay_amount) VALUES(?,?,100,100)", no, u)
			id := scalar("SELECT id FROM askxuan_order.shop_order WHERE order_no=?", no)
			exec("INSERT INTO askxuan_order.shop_order_item(order_id,product_id,sku_id,product_name,price,quantity) VALUES(?,1,991019,'并发测试',100,1)", id)
			r := cashier.OrderRequest{OrderType: "shop_order", OrderNo: no}
			q, e := c.GrowthQuote(ctx, u, r)
			check(e)
			r.QuoteID = q.Pricing.ID
			r.OfferKind = "level"
			r.ExpectedCents = 9800
			requests[i] = r
		}
		errs := make(chan error, 2)
		var wg sync.WaitGroup
		for i, u := range users {
			wg.Add(1)
			go func(u string, r cashier.OrderRequest) { defer wg.Done(); _, e := c.Reserve(ctx, u, r); errs <- e }(u, requests[i])
		}
		wg.Wait()
		close(errs)
		success := 0
		for e := range errs {
			if e == nil {
				success++
			}
		}
		if success != 1 {
			t.Fatalf("budget overspent or lost: successes=%d", success)
		}
		exec("UPDATE growth_settings SET enabled=0")
		now = now.Add(16 * time.Minute)
		check(g.Tick(ctx))
		if scalar("SELECT held_cents FROM growth_budget WHERE kind='level'") != 0 {
			t.Fatal("expired hold stuck while disabled")
		}
		// Feature-off is an ordinary original-price checkout, including payment.
		u := users[0]
		r := requests[0]
		r.QuoteID = ""
		r.OfferKind = ""
		r.ExpectedCents = 10000
		r.Channel = "balance"
		q, e := c.GrowthQuote(ctx, u, r)
		check(e)
		if q.Pricing != nil || q.Cents != 10000 {
			t.Fatal("feature off blocks cashier")
		}
		check(db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
			return balance.Change(ctx, tx, u, "off"+suffix, "test", "test", 10000, 0)
		}))
		paid, e := c.Pay(ctx, u, r)
		check(e)
		if scalar("SELECT COUNT(*) FROM growth_transaction WHERE payment_id=?", paid.Id) != 0 {
			t.Fatal("disabled growth accrued")
		}
		exec("UPDATE growth_settings SET enabled=1")
		exec("UPDATE growth_budget SET total_cents=10000000 WHERE kind='level'")
	})
	t.Run("monthly task cap allows a partial final award without refilling", func(t *testing.T) {
		u := "cap" + suffix
		exec("INSERT INTO growth_account(user_id,total,updated_at) VALUES(?,199,?)", u, now.Unix())
		exec("INSERT INTO growth_event(event_key,user_id,kind,reference_no,original_award,current_award,applied,state,business_day,business_month,rule_version,created_at,updated_at) VALUES(?,?,'learning','fixture',199,199,199,'posted',?,?,'growth-v1',?,?)", u, u, growth.BusinessDay(now), growth.BusinessMonth(now), now.Unix(), now.Unix())
		check(g.SaveNote(ctx, u, "记录今日行动", "", false))
		v, e := g.Overview(ctx, u)
		check(e)
		if v.Total != 200 || v.MonthlyEarned != 200 {
			t.Fatalf("partial cap %+v", v)
		}
		check(g.DeleteNote(ctx, u, growth.BusinessDay(now)))
		check(g.SaveNote(ctx, u, "再次保存", "", false))
		v, e = g.Overview(ctx, u)
		check(e)
		if v.Total != 200 {
			t.Fatal("deleted note refilled quota")
		}
	})

	t.Run("all business source snapshots exclude voluntary giving", func(t *testing.T) {
		u := "sources" + suffix
		bookingNo := "BK" + suffix
		exec(`INSERT INTO askxuan_booking.booking(booking_no,user_id,temple_code,master_code,service_code,service_name,booking_date,time_slot,service_fee,merit_money,total_fee,price_snapshot,payment_expire_time) VALUES(?,1,'T001','M001','S001','固定服务','2026-10-19','09:00',100,50,150,'{"templeFee":40,"masterFee":60}',FROM_UNIXTIME(?))`, bookingNo, now.Unix()+3600)
		diyNo := "DY" + suffix
		exec(`INSERT INTO askxuan_diy.diy_order(order_no,user_id,material_fee,bless_fee,total_fee,pricing_snapshot) VALUES(?,?,100,50,150,'{"consecration":{"providerType":"master","masterCode":"M001","sourceServiceCode":"S001"}}')`, diyNo, u)
		id := scalar("SELECT id FROM askxuan_diy.diy_order WHERE order_no=?", diyNo)
		exec("INSERT INTO askxuan_diy.diy_order_item(order_id,material_id,material_name,unit_price,quantity) VALUES(?,1,'测试材料',50,2)", id)
		consultNo := "CS" + suffix
		exec("INSERT INTO askxuan_booking.consultation_order(order_no,request_id,user_id,master_code,master_name,consult_fee,valid_hours,response_minutes) VALUES(?,?,?,'M001','测试师傅',50,24,30)", consultNo, consultNo, u)
		reportNo := "RP" + suffix
		exec(`INSERT INTO askxuan_ai.ai_report(report_no,user_id,request_key,skill_code,title,version,question,inputs_json,chapters_json,price_cents,points_price,status,summary,content) VALUES(?,?,?,'bazi','测试报告','v1','测试','{}','[]',15000,0,'ready','','')`, reportNo, u, reportNo)
		for _, v := range []struct {
			kind, no             string
			gross, growth, goods int64
		}{{"booking", bookingNo, 15000, 10000, 0}, {"diy_order", diyNo, 15000, 15000, 10000}, {"consultation", consultNo, 5000, 5000, 0}, {"ai_report", reportNo, 15000, 15000, 0}} {
			check(db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
				q, e := g.PreviewTx(ctx, tx, u, v.kind, v.no, v.gross)
				if e == nil && (q.Payable != v.gross || q.GrowthBase != v.growth || q.GoodsNet != v.goods) {
					t.Fatalf("%s snapshot %+v", v.kind, q)
				}
				return e
			}))
		}
		c := cashier.Store{DB: db, Enabled: true, Growth: g}
		if _, e := c.GrowthQuote(ctx, "other", cashier.OrderRequest{OrderType: "diy_order", OrderNo: diyNo}); e == nil {
			t.Fatal("foreign checkout allowed")
		}
		a, e := g.AccountAudit(ctx, u, 1)
		check(e)
		if a["exists"] != true {
			t.Fatal("audit account missing")
		}
		_, e = g.Audit(ctx)
		check(e)
	})

}
