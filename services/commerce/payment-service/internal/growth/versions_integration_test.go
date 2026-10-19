package growth_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/askxuan/payment-service/internal/growth"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestGrowthVersionAndCatalogMySQL(t *testing.T) {
	dsn := os.Getenv("GROWTH_TEST_DSN")
	if dsn == "" {
		t.Skip("isolated MySQL required")
	}
	if os.Getenv("GROWTH_TEST_ISOLATED") != "1" || !strings.Contains(dsn, "tcp(127.0.0.1:") || !strings.Contains(dsn, "/askxuan_payment?") {
		t.Fatal("isolated localhost schema required")
	}
	logx.Disable()
	ctx := context.Background()
	db := sqlx.NewMysql(dsn)
	now := time.Now().AddDate(2, 0, 0)
	g := &growth.Store{DB: db, Now: func() time.Time { return now }}
	check := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	exec := func(q string, args ...any) { t.Helper(); _, e := db.ExecCtx(ctx, q, args...); check(e) }
	count := func(q string, args ...any) int64 {
		t.Helper()
		var n int64
		check(db.QueryRowCtx(ctx, &n, q, args...))
		return n
	}
	suffix := fmt.Sprint(time.Now().UnixNano())
	user := "version" + suffix
	version := "growth-t" + suffix
	t.Cleanup(func() { exec("DELETE FROM growth_rule_release WHERE version LIKE 'growth-t%'") })
	exec("UPDATE growth_settings SET enabled=1,discounts_enabled=1")
	exec("UPDATE growth_budget SET total_cents=10000000")
	exec("INSERT INTO growth_account(user_id,total,updated_at)VALUES(?,1250,?)", user, now.Unix())
	exec("INSERT INTO askxuan_product.product(product_no,name,status,price,stock)VALUES(?,'等级价测试','on_shelf',100,100)", suffix)
	product := count("SELECT id FROM askxuan_product.product WHERE product_no=?", suffix)
	exec("INSERT INTO askxuan_product.product_sku(product_id,price,stock)VALUES(?,100,100)", product)
	sku := count("SELECT id FROM askxuan_product.product_sku WHERE product_id=?", product)
	exec("INSERT INTO growth_item_policy(item_type,item_id,title,owner_type,owner_id,enabled,consent,cost_cents,minimum_contribution_cents,subsidy_cap_cents,updated_at)VALUES('shop_sku',?,'测试','platform','',1,1,0,0,10000,?)", fmt.Sprint(sku), now.Unix())
	items := []growth.CatalogItem{{Type: "shop_sku", ID: fmt.Sprint(sku), Quantity: 2}}
	var original growth.PriceQuote
	order := "GV" + suffix
	exec("INSERT INTO askxuan_order.shop_order(order_no,user_id,total_amount,pay_amount)VALUES(?,?,200,200)", order, user)
	orderID := count("SELECT id FROM askxuan_order.shop_order WHERE order_no=?", order)
	exec("INSERT INTO askxuan_order.shop_order_item(order_id,product_id,sku_id,product_name,price,quantity)VALUES(?,?,?,'测试',100,2)", orderID, product, sku)
	t.Run("catalog and cashier share authoritative prices without reservations", func(t *testing.T) {
		before := count("SELECT COUNT(*) FROM growth_quote WHERE user_id=?", user)
		preview, e := g.CatalogPrice(ctx, user, items)
		check(e)
		if preview.Gross != 20000 || preview.LevelPrice == nil || preview.LevelPrice.Payable != 19400 || preview.Snapshot.Selected.Growth != 194 {
			t.Fatalf("preview %+v", preview)
		}
		if count("SELECT COUNT(*) FROM growth_quote WHERE user_id=?", user) != before {
			t.Fatal("catalog preview mutated orders")
		}
		check(db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
			var e error
			original, e = g.PreviewTx(ctx, tx, user, "shop_order", order, 20000)
			return e
		}))
		// A funded upgrade coupon may be the cashier's best option. The level offer must match independently.
		found := false
		for _, o := range original.Snapshot.Offers {
			if o.Kind == "level" {
				found = true
				if o.Payable != preview.LevelPrice.Payable {
					t.Fatal("catalog/cashier mismatch")
				}
			}
		}
		if !found {
			t.Fatal("missing level offer")
		}
		exec("UPDATE growth_settings SET discounts_enabled=0")
		p, e := g.CatalogPrice(ctx, user, items)
		check(e)
		if p.LevelPrice != nil || p.Snapshot.Selected.Payable != 20000 {
			t.Fatal("disabled discount leaked")
		}
		exec("UPDATE growth_settings SET discounts_enabled=1")
		exec("UPDATE growth_budget SET total_cents=held_cents+spent_cents WHERE kind='level'")
		p, e = g.CatalogPrice(ctx, user, items)
		check(e)
		if p.LevelPrice != nil {
			t.Fatal("unfunded price promised")
		}
		exec("UPDATE growth_budget SET total_cents=10000000 WHERE kind='level'")
		bad := append([]growth.CatalogItem{}, items...)
		bad[0].Quantity = 0
		if _, e = g.CatalogPrice(ctx, user, bad); e == nil {
			t.Fatal("zero quantity accepted")
		}
		if _, e = g.CatalogPrice(ctx, user, append(items, items...)); e == nil {
			t.Fatal("duplicate items accepted")
		}
		exec("UPDATE askxuan_product.product SET status='off_shelf' WHERE id=?", product)
		if _, e = g.CatalogPrice(ctx, user, items); e == nil {
			t.Fatal("off-shelf price accepted")
		}
		exec("UPDATE askxuan_product.product SET status='on_shelf' WHERE id=?", product)
	})
	t.Run("catalog enforces material specifications and private frozen report price", func(t *testing.T) {
		exec("UPDATE askxuan_product.product SET is_experience=1 WHERE id=?", product)
		if _, e := g.CatalogPrice(ctx, user, items); e == nil {
			t.Fatal("experience price accepted")
		}
		exec("UPDATE askxuan_product.product SET is_experience=0 WHERE id=?", product)
		exec("INSERT INTO askxuan_diy.material(name,spec,unit_price,stock) VALUES(?,'10mm',10,10)", suffix)
		material := count("SELECT id FROM askxuan_diy.material WHERE name=?", suffix)
		exec("INSERT INTO askxuan_diy.material_sku(material_id,spec,price,stock) VALUES(?,'12mm',25,3)", material)
		materialSKU := count("SELECT id FROM askxuan_diy.material_sku WHERE material_id=?", material)
		selected := growth.CatalogItem{Type: "material", ID: fmt.Sprint(material), Spec: "12mm", Quantity: 2}
		p, e := g.CatalogPrice(ctx, user, []growth.CatalogItem{selected})
		check(e)
		if p.Gross != 5000 {
			t.Fatalf("selected SKU price ignored: %d", p.Gross)
		}
		if _, e = g.CatalogPrice(ctx, user, []growth.CatalogItem{selected, {Type: "material_sku", ID: fmt.Sprint(materialSKU), Quantity: 1}}); e == nil {
			t.Fatal("same resolved SKU double counted")
		}
		selected.Quantity = 4
		if _, e = g.CatalogPrice(ctx, user, []growth.CatalogItem{selected}); e == nil {
			t.Fatal("SKU stock ignored")
		}
		selected.Quantity = 1
		selected.Spec = "invalid"
		if _, e = g.CatalogPrice(ctx, user, []growth.CatalogItem{selected}); e == nil {
			t.Fatal("unknown spec accepted")
		}
		reportNo := "AR" + suffix
		exec("INSERT INTO askxuan_ai.ai_report(report_no,user_id,request_key,skill_code,title,version,question,inputs_json,chapters_json,price_cents,points_price,summary,content) VALUES(?,?,?,'bazi','测试报告','v1','',JSON_OBJECT(),JSON_ARRAY(),1234,20,'','')", reportNo, user, suffix)
		reportItems := []growth.CatalogItem{{Type: "ai_report_order", ID: reportNo, Quantity: 1}}
		p, e = g.CatalogPrice(ctx, user, reportItems)
		check(e)
		if p.Gross != 1234 {
			t.Fatal("report frozen price ignored")
		}
		if _, e = g.CatalogPrice(ctx, "other-user", reportItems); e == nil {
			t.Fatal("private report disclosed")
		}
		exec("UPDATE askxuan_ai.ai_report SET points_paid=1 WHERE report_no=?", reportNo)
		if _, e = g.CatalogPrice(ctx, user, reportItems); e == nil {
			t.Fatal("paid report offered again")
		}
	})
	t.Run("cross device notice claims collapse skipped levels once", func(t *testing.T) {
		var hits atomic.Int32
		var wg sync.WaitGroup
		errs := make(chan error, 16)
		for range 8 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				n, e := g.ClaimLevelNotice(ctx, user)
				errs <- e
				if n != nil {
					if n.Level.Code != 3 {
						errs <- fmt.Errorf("wrong final level")
					}
					hits.Add(1)
				}
			}()
		}
		wg.Wait()
		close(errs)
		for e := range errs {
			check(e)
		}
		if hits.Load() != 1 {
			t.Fatalf("notice count %d", hits.Load())
		}
		exec("UPDATE growth_account SET total=199 WHERE user_id=?", user)
		n, e := g.ClaimLevelNotice(ctx, user)
		check(e)
		if n != nil {
			t.Fatal("downgrade emitted upgrade")
		}
		exec("UPDATE growth_account SET total=1250 WHERE user_id=?", user)
	})
	t.Run("two-person scheduled rules activate exactly and preserve old quotes", func(t *testing.T) {
		rules := growth.DefaultRules()
		rules.Levels[2].GoodsRate = 9600
		rules.Tasks[2].Award = 4
		p := growth.RuleProposal{Version: version, BaseVersion: growth.RuleVersion, EffectiveAt: now.Unix() + 120, Summary: "测试定时规则，验证新旧订单边界", Config: rules}
		_, e := g.RulePreview(ctx, p)
		check(e)
		raw, _ := json.Marshal(p)
		id, e := g.Propose(ctx, "admin-a", "rules", "测试定时发布的独立复核", raw)
		check(e)
		if e = g.Approve(ctx, "admin-a", id, true); e == nil {
			t.Fatal("self approval allowed")
		}
		check(g.Approve(ctx, "admin-b", id, true))
		history, e := g.Rules(ctx, 1, false)
		check(e)
		if history["currentVersion"] != growth.RuleVersion {
			t.Fatal("future rules leaked")
		}
		cfg, e := g.Settings(ctx)
		check(e)
		if cfg.Version != growth.RuleVersion {
			t.Fatal("early activation")
		}
		now = time.Unix(p.EffectiveAt, 0)
		cfg, e = g.Settings(ctx)
		check(e)
		if cfg.Version != version {
			t.Fatal("activation requires another client or worker")
		}
		v, e := g.Overview(ctx, user)
		check(e)
		if v.Level.GoodsRate != 9600 || v.Version != version {
			t.Fatal("overview stale rules")
		}
		preview, e := g.CatalogPrice(ctx, user, items)
		check(e)
		if preview.LevelPrice == nil || preview.LevelPrice.Payable != 19200 {
			t.Fatalf("new price %+v", preview)
		}
		if original.Snapshot.Level.GoodsRate != 9700 || original.Version != growth.RuleVersion {
			t.Fatal("old quote was rewritten")
		}
		e = db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
			_, e := g.ReserveTx(ctx, tx, user, "shop_order", order, original.ID, original.SelectedKind, original.CouponID, original.Payable, 20000)
			return e
		})
		if e == nil {
			t.Fatal("stale preview accepted after activation")
		}
		check(g.SaveNote(ctx, user, "版本生效后的行动记录", "", false))
		v, e = g.Overview(ctx, user)
		check(e)
		if v.Total != 1254 {
			t.Fatalf("new task arithmetic not effective: %d", v.Total)
		}
		if count("SELECT COUNT(*) FROM growth_event WHERE user_id=? AND rule_version=?", user, version) != 1 {
			t.Fatal("event missing rule version")
		}
		history, e = g.Rules(ctx, 1, false)
		check(e)
		if history["currentVersion"] != version || len(history["items"].([]growth.RuleRelease)) < 2 {
			t.Fatal("history missing")
		}
		p.Version = version + "x"
		p.BaseVersion = version
		p.EffectiveAt = now.Unix() + 120
		raw, _ = json.Marshal(p)
		id, e = g.Propose(ctx, "admin-a", "rules", "测试取消未来规则的独立复核", raw)
		check(e)
		check(g.Approve(ctx, "admin-b", id, true))
		raw, _ = json.Marshal(map[string]string{"version": p.Version})
		id, e = g.Propose(ctx, "admin-a", "rule_cancel", "测试取消未生效版本", raw)
		check(e)
		check(g.Approve(ctx, "admin-b", id, true))
		now = now.Add(180 * time.Second)
		cfg, e = g.Settings(ctx)
		check(e)
		if cfg.Version != version {
			t.Fatal("cancelled rule activated")
		}
	})
}
