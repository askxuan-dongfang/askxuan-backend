package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func publicationDocument(ids ...int64) string {
	beads := make([]map[string]any, 0, len(ids))
	for i, id := range ids {
		beads = append(beads, map[string]any{"materialId": id, "spec": "10mm", "slotId": fmt.Sprint(i), "unitPrice": .01})
	}
	raw, _ := json.Marshal(map[string]any{"version": 2, "wristSizeMm": 160, "fitAllowanceMm": 8, "beads": beads})
	return string(raw)
}

func TestPublicationFingerprint(t *testing.T) {
	base := publicationDocument(1, 2)
	hash, err := CompositionFingerprint(base)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []struct {
		name, raw string
		same      bool
	}{
		{"slot identifiers are presentation only", strings.ReplaceAll(base, `"slotId":"0"`, `"slotId":"other"`), true},
		{"client price is not an identity", strings.ReplaceAll(base, `"unitPrice":0.01`, `"unitPrice":999`), true},
		{"bead order changes style", publicationDocument(2, 1), false},
		{"material changes style", publicationDocument(1, 3), false},
		{"spec changes style", strings.Replace(base, `"spec":"10mm"`, `"spec":"8mm"`, 1), false},
		{"wrist changes style", strings.Replace(base, `"wristSizeMm":160`, `"wristSizeMm":170`, 1), false},
		{"allowance changes style", strings.Replace(base, `"fitAllowanceMm":8`, `"fitAllowanceMm":10`, 1), false},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			got, err := CompositionFingerprint(mutation.raw)
			if err != nil || (got == hash) != mutation.same {
				t.Fatalf("hash %s error %v", got, err)
			}
		})
	}
	for _, invalid := range []string{`{}`, `[]`, `{"version":2,"beads":[]}`, `{"version":1,"items":[{"materialId":1,"quantity":1}]}`} {
		if _, err := CompositionFingerprint(invalid); err == nil {
			t.Fatalf("accepted missing style %s", invalid)
		}
	}
}

func TestConfirmedPublicationBill(t *testing.T) {
	d := DiyDesign{Id: 1, UserId: "123", Revision: 2, DesignData: publicationDocument(1, 2), TotalPrice: .02}
	dJSON, _ := json.Marshal(d)
	p := pricingSnapshot{MaterialFee: 50, TotalFee: 50, Items: []*DiyOrderItem{{MaterialId: 1, Spec: "10mm", Quantity: 1, UnitPrice: 20}, {MaterialId: 2, Spec: "10mm", Quantity: 1, UnitPrice: 30}}}
	pJSON, _ := json.Marshal(p)
	order := DiyOrder{DesignId: 1, UserId: "123", MaterialFee: 50, TotalFee: 50, DesignSnapshot: string(dJSON), PricingSnapshot: string(pJSON)}
	confirmed, _, err := confirmedOrderDesign(&order)
	if err != nil || confirmed.TotalPrice != 50 || !strings.Contains(confirmed.DesignData, `"unitPrice":20`) {
		t.Fatalf("confirmed %+v %v", confirmed, err)
	}
	for _, mutate := range []struct {
		name string
		run  func(*DiyOrder)
	}{
		{"wrong owner", func(o *DiyOrder) { o.UserId = "999" }},
		{"wrong design", func(o *DiyOrder) { o.DesignId = 9 }},
		{"wrong total", func(o *DiyOrder) { o.TotalFee = 49 }},
		{"wrong fee", func(o *DiyOrder) { o.MaterialFee = 49 }},
		{"missing snapshot", func(o *DiyOrder) { o.DesignSnapshot = "" }},
		{"wrong material", func(o *DiyOrder) {
			o.PricingSnapshot = strings.Replace(o.PricingSnapshot, `"materialId":2`, `"materialId":3`, 1)
		}},
		{"wrong quantity", func(o *DiyOrder) {
			o.PricingSnapshot = strings.Replace(o.PricingSnapshot, `"quantity":1`, `"quantity":2`, 1)
		}},
		{"negative price", func(o *DiyOrder) {
			o.PricingSnapshot = strings.Replace(o.PricingSnapshot, `"unitPrice":20`, `"unitPrice":-20`, 1)
		}},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			o := order
			mutate.run(&o)
			if _, _, err := confirmedOrderDesign(&o); err == nil {
				t.Fatal("inconsistent paid snapshot accepted")
			}
		})
	}
}

func TestLegacyDraftCheckoutDoesNotGrantPublication(t *testing.T) {
	item := `{"materialId":1,"spec":"10mm","quantity":2}`
	inputs := []PricedOrderItemInput{{MaterialId: 1, Spec: "10mm", Quantity: 2}}
	for _, raw := range []string{"[" + item + "]", `{"version":1,"items":[` + item + "]}", `{"materials":[` + item + "]}"} {
		if !matchesLegacyCheckoutItems(raw, inputs) {
			t.Fatalf("legacy bill rejected: %s", raw)
		}
		if _, e := CompositionFingerprint(raw); e == nil {
			t.Fatal("bill-only draft qualified as a complete style")
		}
		if matchesLegacyCheckoutItems(raw, []PricedOrderItemInput{{MaterialId: 1, Spec: "10mm", Quantity: 3}}) {
			t.Fatal("forged legacy quantity accepted")
		}
	}
	for _, raw := range []string{`{"version":2,"items":[` + item + "]}", `{"beads":[],"items":[` + item + "]}", `{"items":[]}`} {
		if matchesLegacyCheckoutItems(raw, inputs) {
			t.Fatal("invalid modern document fell back to legacy", raw)
		}
	}
}

// Run only via the task's disposable local database script. No production DSN,
// credentials or existing application database is accepted.
func TestPublicationMySQL(t *testing.T) {
	dsn := os.Getenv("DIY_PUBLICATION_TEST_DSN")
	if dsn == "" {
		t.Skip("requires disposable publication MySQL")
	}
	if os.Getenv("DIY_PUBLICATION_TEST_ISOLATED") != "1" || !strings.Contains(dsn, "tcp(127.0.0.1:") || !strings.Contains(dsn, "/askxuan_diy?") {
		t.Fatal("isolated localhost only")
	}
	logx.Disable()
	ctx := context.Background()
	db := sqlx.NewMysql(dsn)
	designs, orders, orderItems := NewDiyDesignModel(db), NewDiyOrderModel(db), NewDiyOrderItemModel(db)
	nonce := fmt.Sprintf("PUB%d", time.Now().UnixNano())
	var seq int
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := db.ExecCtx(ctx, q, args...); e != nil {
			t.Fatal(e)
		}
	}
	material, err := NewMaterialModel(db).Insert(ctx, &Material{Name: nonce, Spec: "10mm", UnitPrice: 20, Stock: 500, Category: "main_bead", Status: MaterialStatusOnShelf})
	if err != nil {
		t.Fatal(err)
	}
	exec("UPDATE askxuan_diy.creator_reward_policy SET enabled=1,kind='cash',rate_bps=500,cap_cents=2000,wait_days=7")
	newDesign := func(user string) *DiyDesign {
		t.Helper()
		seq++
		d, e := designs.Insert(ctx, &DiyDesign{UserId: user, Name: fmt.Sprintf("%s-%d", nonce, seq), DesignData: publicationDocument(material.Id), TotalPrice: 20, Status: DesignStatusPrivate})
		if e != nil {
			t.Fatal(e)
		}
		return d
	}
	ownOrder := func(d *DiyDesign, status, payment, channel string) *DiyOrder {
		t.Helper()
		seq++
		raw, _ := json.Marshal(d)
		c, e := readComposition(d.DesignData)
		if e != nil {
			t.Fatal(e)
		}
		items := []*DiyOrderItem{}
		fee := 0.0
		for key, qty := range compositionBill(c) {
			items = append(items, &DiyOrderItem{MaterialId: key.MaterialId, MaterialName: nonce, Spec: key.Spec, Quantity: qty, UnitPrice: 20})
			fee += 20 * float64(qty)
		}
		price, _ := json.Marshal(pricingSnapshot{MaterialFee: fee, TotalFee: fee, Items: items})
		o, e := orders.Insert(ctx, &DiyOrder{OrderNo: fmt.Sprintf("%s%d", nonce, seq), UserId: d.UserId, DesignId: d.Id, MaterialFee: fee, TotalFee: fee, Status: status, PaymentStatus: payment, AddressId: 1, DesignSnapshot: string(raw), PricingSnapshot: string(price)})
		if e != nil {
			t.Fatal(e)
		}
		if channel != "" {
			exec("INSERT INTO askxuan_payment.payment(payment_no,user_id,order_type,order_no,amount,channel,status) VALUES(?,?,'diy_order',?,?,?,'success')", o.OrderNo, d.UserId, o.OrderNo, o.TotalFee, channel)
		}
		return o
	}
	checkReason := func(d *DiyDesign, want string) {
		t.Helper()
		e, _, err := ReadPublicationEligibility(ctx, db, d, false)
		if err != nil || e.Reason != want || e.CanPublish != (want == "ready") {
			t.Fatalf("reason=%+v want=%s err=%v", e, want, err)
		}
	}
	publish := func(d *DiyDesign) *DiyDesign {
		t.Helper()
		updated, e := ChangePublicationStatus(ctx, db, d.Id, d.Revision, d.UserId, DesignStatusPublic, false)
		if e != nil {
			t.Fatal(e)
		}
		return updated
	}
	copyOf := func(d *DiyDesign, user string) *DiyDesign {
		t.Helper()
		p, _, e := PublishedDesign(ctx, db, d)
		if e != nil {
			t.Fatal(e)
		}
		out, e := designs.Insert(ctx, &DiyDesign{UserId: user, Name: "copy-" + nonce, DesignData: p.DesignData, TotalPrice: p.TotalPrice, Status: DesignStatusPrivate, SourceDesignId: d.Id, SourcePublicationId: p.PublicationId})
		if e != nil {
			t.Fatal(e)
		}
		return out
	}
	checkoutResult := func(ctx context.Context, d *DiyDesign, user, source string) (*PricedOrderResult, error) {
		items := []PricedOrderItemInput{}
		c, e := readComposition(d.DesignData)
		if e != nil {
			return nil, e
		}
		for key, qty := range compositionBill(c) {
			items = append(items, PricedOrderItemInput{MaterialId: key.MaterialId, Spec: key.Spec, Quantity: qty, SnapshotUnitPrice: 20})
		}
		return CreatePricedOrder(ctx, db, orders, orderItems, PricedOrderInput{UserId: user, Design: d, Items: items, AddressId: 1, Source: source, CreatorId: "forged"})
	}
	checkout := func(d *DiyDesign, user, source string) *DiyOrder {
		t.Helper()
		res, e := checkoutResult(ctx, d, user, source)
		if e != nil {
			t.Fatal(e)
		}
		return res.Order
	}
	t.Run("qualification states and owner binding", func(t *testing.T) {
		d := newDesign("990910")
		checkReason(d, "order_required")
		if _, e := ChangePublicationStatus(ctx, db, d.Id, d.Revision, d.UserId, DesignStatusPublic, false); e == nil {
			t.Fatal("unpaid draft published")
		}
		foreign := *d
		foreign.UserId = "990911"
		ownOrder(&foreign, DiyStatusInMaking, "success", "alipay")
		checkReason(d, "order_required")
		mine, _, err := designs.FindListByUserWithOrders(ctx, d.UserId, 1, 100)
		if err != nil || len(mine) == 0 || mine[0].OrderNo != "" {
			t.Fatal("mine exposed another buyer's order", err)
		}
		o := ownOrder(d, DiyStatusPendingReview, "pending", "")
		ownOrder(&foreign, DiyStatusInMaking, "success", "alipay")
		mine, _, err = designs.FindListByUserWithOrders(ctx, d.UserId, 1, 100)
		if err != nil || len(mine) == 0 || mine[0].OrderNo != o.OrderNo {
			t.Fatal("new foreign order replaced owner's order progress", err)
		}
		checkReason(d, "payment_required")
		exec("UPDATE diy_order SET payment_status='success' WHERE id=?", o.Id)
		exec("INSERT INTO askxuan_payment.payment(payment_no,user_id,order_type,order_no,amount,channel,status) VALUES(?,?,'diy_order',?,20,'mock','success')", o.OrderNo, d.UserId, o.OrderNo)
		checkReason(d, "payment_required")
		exec("UPDATE askxuan_payment.payment SET channel='alipay' WHERE order_no=?", o.OrderNo)
		checkReason(d, "confirmation_required")
		exec("UPDATE diy_order SET status='in_making' WHERE id=?", o.Id)
		checkReason(d, "ready")
		for _, s := range []string{DiyStatusCancelled, DiyStatusInReturn} {
			exec("UPDATE diy_order SET status=? WHERE id=?", s, o.Id)
			checkReason(d, "order_inactive")
		}
		exec("UPDATE diy_order SET status='in_making' WHERE id=?", o.Id)
		for _, s := range []string{"refunding", "refunded"} {
			exec("UPDATE diy_order SET payment_status=? WHERE id=?", s, o.Id)
			checkReason(d, "order_inactive")
		}
		exec("UPDATE diy_order SET payment_status='success' WHERE id=?", o.Id)
		exec("INSERT INTO askxuan_payment.refund(refund_no,payment_id,amount,status) SELECT ?,id,20,'pending' FROM askxuan_payment.payment WHERE order_no=?", nonce+"refund", o.OrderNo)
		checkReason(d, "order_inactive")
		exec("UPDATE askxuan_payment.refund SET status='failed' WHERE refund_no=?", nonce+"refund")
		checkReason(d, "ready")
		d.Status = DesignStatusRejected
		checkReason(d, "moderation_required")
		d.Status = DesignStatusPrivate
		changed := *d
		changed.DesignData = publicationDocument(material.Id, material.Id)
		checkReason(&changed, "design_changed")
		exec("UPDATE diy_order SET pricing_snapshot='{}' WHERE id=?", o.Id)
		checkReason(d, "snapshot_unavailable")
	})
	t.Run("snapshot stays fixed through draft changes and revision conflicts", func(t *testing.T) {
		d := newDesign("990920")
		ownOrder(d, DiyStatusInMaking, "success", "alipay")
		originalRevision := d.Revision
		d = publish(d)
		if _, err := ChangePublicationStatus(ctx, db, d.Id, originalRevision, d.UserId, DesignStatusPrivate, false); err == nil {
			t.Fatal("stale mutation accepted")
		}
		publicBefore, p, e := PublishedDesign(ctx, db, d)
		if e != nil {
			t.Fatal(e)
		}
		d.Name = "updated draft"
		d.DesignData = publicationDocument(material.Id, material.Id)
		d.TotalPrice = 40
		if e := designs.UpdateOwned(ctx, d, d.Revision); e != nil {
			t.Fatal(e)
		}
		publicAfter, after, e := PublishedDesign(ctx, db, d)
		if e != nil || after.DesignSnapshot != p.DesignSnapshot || publicAfter.TotalPrice != 20 || publicAfter.DesignData != publicBefore.DesignData {
			t.Fatal("public snapshot changed", e)
		}
		if !DraftDiffers(d, after) {
			t.Fatal("missing changed draft")
		}
		checkReason(d, "design_changed")
		ownOrder(d, DiyStatusInMaking, "success", "alipay")
		next := publish(d)
		if next.PublicationId == p.Id {
			t.Fatal("mutated old publication")
		}
		var stored string
		if e := db.QueryRowCtx(ctx, &stored, "SELECT design_snapshot FROM diy_design_publication WHERE id=?", p.Id); e != nil || stored != p.DesignSnapshot {
			t.Fatal("old immutable version changed", e)
		}
	})
	t.Run("concurrent publish is compare and swap", func(t *testing.T) {
		d := newDesign("990930")
		ownOrder(d, DiyStatusInMaking, "success", "alipay")
		var success atomic.Int32
		var wg sync.WaitGroup
		for i := 0; i < 4; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := ChangePublicationStatus(ctx, db, d.Id, d.Revision, d.UserId, DesignStatusPublic, false)
				if err == nil {
					success.Add(1)
				} else {
					var biz *common.BizError
					if !errors.As(err, &biz) || biz.Code != 40914 {
						t.Errorf("unexpected error %v", err)
					}
				}
			}()
		}
		wg.Wait()
		if success.Load() != 1 {
			t.Fatalf("published %d times", success.Load())
		}
	})
	t.Run("checkout rejects stale revision and forged bill without reserving stock", func(t *testing.T) {
		d := newDesign("990935")
		stale := *d
		d.Name = "renamed"
		if e := designs.UpdateOwned(ctx, d, d.Revision); e != nil {
			t.Fatal(e)
		}
		var stockBefore, stockAfter, orderCount int64
		if e := db.QueryRowCtx(ctx, &stockBefore, "SELECT stock FROM material WHERE id=?", material.Id); e != nil {
			t.Fatal(e)
		}
		for _, tc := range []struct {
			design *DiyDesign
			qty    int
			code   int
		}{{&stale, 1, 40914}, {d, 2, 40915}} {
			_, err := CreatePricedOrder(ctx, db, orders, orderItems, PricedOrderInput{UserId: d.UserId, Design: tc.design,
				Items: []PricedOrderItemInput{{MaterialId: material.Id, Spec: "10mm", Quantity: tc.qty}}, AddressId: 1, Source: "custom"})
			var biz *common.BizError
			if !errors.As(err, &biz) || biz.Code != tc.code {
				t.Fatal("unexpected checkout rejection", err)
			}
		}
		if e := db.QueryRowCtx(ctx, &stockAfter, "SELECT stock FROM material WHERE id=?", material.Id); e != nil || stockAfter != stockBefore {
			t.Fatal("rejected checkout changed stock", e)
		}
		if e := db.QueryRowCtx(ctx, &orderCount, "SELECT COUNT(*) FROM diy_order WHERE design_id=?", d.Id); e != nil || orderCount != 0 {
			t.Fatal("rejected checkout inserted order", e)
		}
	})
	t.Run("copy lineage freezes original beneficiary and refund cannot be laundered", func(t *testing.T) {
		a := newDesign("990940")
		aOrder := ownOrder(a, DiyStatusInMaking, "success", "alipay")
		a = publish(a)
		aV1 := a.PublicationId
		b := copyOf(a, "990941")
		checkReason(b, "order_required")
		bo := checkout(b, b.UserId, "custom")
		if bo.CreatorId != a.UserId {
			t.Fatalf("wrong B beneficiary %s", bo.CreatorId)
		}
		ownOrder(b, DiyStatusInMaking, "success", "alipay")
		b = publish(b)
		c := copyOf(b, "990942")
		co := checkout(c, c.UserId, "custom")
		if co.CreatorId != a.UserId {
			t.Fatalf("copy chain washed creator %s", co.CreatorId)
		}
		pubID, designID := publicationSnapshotForReward(co.DesignSnapshot)
		if pubID != aV1 || designID != a.Id {
			t.Fatal("missing immutable root binding")
		}
		var rewardRoot int64
		if e := db.QueryRowCtx(ctx, &rewardRoot, "SELECT source_publication_id FROM creator_reward WHERE order_id=?", co.Id); e != nil || rewardRoot != aV1 {
			t.Fatal("reward lost root version", e)
		}
		oldSnapshot := co.DesignSnapshot
		a.Name = "new draft"
		a.DesignData = publicationDocument(material.Id, material.Id)
		a.TotalPrice = 40
		if e := designs.UpdateOwned(ctx, a, a.Revision); e != nil {
			t.Fatal(e)
		}
		ownOrder(a, DiyStatusInMaking, "success", "alipay")
		a = publish(a)
		if a.PublicationId == aV1 {
			t.Fatal("new version reused old identity")
		}
		if _, e := FindPublication(ctx, db, aV1, false); !errors.Is(e, sqlx.ErrNotFound) {
			t.Fatal("old version exposed as current public detail", e)
		}
		if _, e := FindAttributionPublication(ctx, db, aV1, false); e != nil {
			t.Fatal("valid replaced version lost attribution", e)
		}
		afterReplacement := copyOf(b, "990946")
		o := checkout(afterReplacement, afterReplacement.UserId, "custom")
		id, _ := publicationSnapshotForReward(o.DesignSnapshot)
		if o.CreatorId != a.UserId || id != aV1 {
			t.Fatal("A-v2 replacement rewrote B's A-v1 attribution")
		}
		var e error
		a, e = ChangePublicationStatus(ctx, db, a.Id, a.Revision, a.UserId, DesignStatusPrivate, false)
		if e != nil {
			t.Fatal(e)
		}
		afterUnpublish := copyOf(b, "990947")
		if o := checkout(afterUnpublish, afterUnpublish.UserId, "custom"); o.CreatorId != "" {
			t.Fatal("explicitly unpublished root still earns new attribution")
		}
		a = publish(a)
		exec("UPDATE diy_order SET payment_status='refunded' WHERE id=?", aOrder.Id)
		if _, e := FindAttributionPublication(ctx, db, aV1, false); !errors.Is(e, sqlx.ErrNotFound) {
			t.Fatal("refunded root is still attributable", e)
		}
		if _, _, e := PublishedDesign(ctx, db, a); e != nil {
			t.Fatal("refund of A-v1 hid independently confirmed A-v2", e)
		}
		newCopy := copyOf(b, "990943")
		noReward := checkout(newCopy, newCopy.UserId, "custom")
		if noReward.CreatorId != "" {
			t.Fatal("refunded root regained attribution")
		}
		// Even a freshly republished same-style copy retains the revoked root.
		b = publish(b)
		newCopy = copyOf(b, "990944")
		if o := checkout(newCopy, newCopy.UserId, "custom"); o.CreatorId != "" {
			t.Fatal("republish washed invalid root")
		}
		stored, e := orders.FindOne(ctx, co.Id)
		if e != nil || stored.DesignSnapshot != oldSnapshot || stored.CreatorId != co.CreatorId {
			t.Fatal("existing order changed after source edit/unpublish/refund", e)
		}
		if e := db.QueryRowCtx(ctx, &rewardRoot, "SELECT source_publication_id FROM creator_reward WHERE order_id=?", co.Id); e != nil || rewardRoot != aV1 {
			t.Fatal("existing reward version changed after source lifecycle", e)
		}
		// A distinct, newly paid/approved style is B's independent new work.
		b.DesignData = publicationDocument(material.Id, material.Id)
		b.TotalPrice = 40
		if e := designs.UpdateOwned(ctx, b, b.Revision); e != nil {
			t.Fatal(e)
		}
		changed := checkout(b, b.UserId, "custom")
		if changed.CreatorId != "" {
			t.Fatal("edited copy attributed before own confirmation")
		}
		ownOrder(b, DiyStatusInMaking, "success", "alipay")
		b = publish(b)
		newCopy = copyOf(b, "990945")
		if o := checkout(newCopy, newCopy.UserId, "custom"); o.CreatorId != b.UserId {
			t.Fatalf("new confirmed style creator=%s", o.CreatorId)
		}
		self, _, e := PublishedDesign(ctx, db, b)
		if e != nil {
			t.Fatal(e)
		}
		if o := checkout(self, b.UserId, "design_square"); o.CreatorId != "" {
			t.Fatal("self purchase rewarded")
		}
	})
	t.Run("legacy public and foreign publication mutation fail closed", func(t *testing.T) {
		d := newDesign("990950")
		exec("UPDATE diy_design SET status='public' WHERE id=?", d.Id)
		d.Status = DesignStatusPublic
		if _, _, e := PublishedDesign(ctx, db, d); e == nil {
			t.Fatal("legacy public leaked")
		}
		if _, e := ChangePublicationStatus(ctx, db, d.Id, d.Revision, "990951", DesignStatusPublic, false); e == nil {
			t.Fatal("foreign publisher accepted")
		}
	})
	t.Run("direct refund serializes with publication and new attribution", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		waitForLock := func(schema, table string) {
			t.Helper()
			for {
				var count int
				err := db.QueryRowCtx(ctx, &count, `SELECT COUNT(*) FROM performance_schema.data_lock_waits w
 JOIN performance_schema.data_locks l ON l.ENGINE_LOCK_ID=w.REQUESTING_ENGINE_LOCK_ID AND l.ENGINE=w.ENGINE
 WHERE l.OBJECT_SCHEMA=? AND l.OBJECT_NAME=?`, schema, table)
				if err != nil {
					t.Fatal(err)
				}
				if count > 0 {
					return
				}
				select {
				case <-ctx.Done():
					t.Fatal("expected transaction lock wait:", schema, table)
				case <-time.After(10 * time.Millisecond):
				}
			}
		}
		hold := func(query string, arg any, after func(sqlx.Session) error) (func(), <-chan error) {
			t.Helper()
			locked, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
			var once sync.Once
			unlock := func() { once.Do(func() { close(release) }) }
			t.Cleanup(unlock)
			go func() {
				done <- db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
					var id int64
					if e := tx.QueryRowCtx(ctx, &id, query, arg); e != nil {
						return e
					}
					close(locked)
					select {
					case <-release:
					case <-ctx.Done():
						return ctx.Err()
					}
					if after != nil {
						return after(tx)
					}
					return nil
				})
			}()
			select {
			case <-locked:
			case e := <-done:
				t.Fatal("lock setup failed", e)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			return unlock, done
		}
		refund := func(o *DiyOrder) func(sqlx.Session) error {
			return func(tx sqlx.Session) error {
				// Mirror the payment-locked critical section of a direct balance
				// refund. Deliberately leave diy_order untouched to expose the race.
				if _, e := tx.ExecCtx(ctx, `INSERT INTO askxuan_payment.refund(refund_no,payment_id,amount,status)
 SELECT CONCAT(payment_no,'R'),id,amount,'success' FROM askxuan_payment.payment WHERE order_no=?`, o.OrderNo); e != nil {
					return e
				}
				_, e := tx.ExecCtx(ctx, "UPDATE askxuan_payment.payment SET status='refunded' WHERE order_no=?", o.OrderNo)
				return e
			}
		}
		await := func(done <-chan error) {
			t.Helper()
			select {
			case e := <-done:
				if e != nil {
					t.Fatal(e)
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		}
		paymentLock := "SELECT id FROM askxuan_payment.payment WHERE order_no=? FOR UPDATE"
		d := newDesign("990960")
		o := ownOrder(d, DiyStatusInMaking, "success", "balance")
		release, refunded := hold(paymentLock, o.OrderNo, refund(o))
		publishDone := make(chan error, 1)
		go func() {
			_, e := ChangePublicationStatus(ctx, db, d.Id, d.Revision, d.UserId, DesignStatusPublic, false)
			var biz *common.BizError
			if !errors.As(e, &biz) || biz.Code != 40915 {
				publishDone <- fmt.Errorf("refund winner must reject publishing: %v", e)
				return
			}
			publishDone <- nil
		}()
		waitForLock("askxuan_payment", "payment")
		release()
		await(refunded)
		await(publishDone)

		a := newDesign("990961")
		aOrder := ownOrder(a, DiyStatusInMaking, "success", "balance")
		a = publish(a)
		b := copyOf(a, "990962")
		release, refunded = hold(paymentLock, aOrder.OrderNo, refund(aOrder))
		checkoutDone := make(chan error, 1)
		go func() {
			result, e := checkoutResult(ctx, b, b.UserId, "custom")
			if e == nil && result.Order.CreatorId != "" {
				e = errors.New("refund winner still gained new attribution")
			}
			checkoutDone <- e
		}()
		waitForLock("askxuan_payment", "payment")
		release()
		await(refunded)
		await(checkoutDone)

		a = newDesign("990963")
		aOrder = ownOrder(a, DiyStatusInMaking, "success", "balance")
		a = publish(a)
		b = copyOf(a, "990964")
		releaseStock, stockDone := hold("SELECT id FROM askxuan_diy.material WHERE id=? FOR UPDATE", material.Id, nil)
		go func() {
			result, e := checkoutResult(ctx, b, b.UserId, "custom")
			if e == nil && result.Order.CreatorId != a.UserId {
				e = errors.New("checkout winner lost its valid frozen attribution")
			}
			checkoutDone <- e
		}()
		waitForLock("askxuan_diy", "material")
		refundDone := make(chan error, 1)
		go func() {
			refundDone <- db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
				var id int64
				if e := tx.QueryRowCtx(ctx, &id, paymentLock, aOrder.OrderNo); e != nil {
					return e
				}
				return refund(aOrder)(tx)
			})
		}()
		waitForLock("askxuan_payment", "payment")
		releaseStock()
		await(stockDone)
		await(checkoutDone)
		await(refundDone)
		if later := checkout(b, b.UserId, "custom"); later.CreatorId != "" {
			t.Fatal("checkout after committed refund gained new attribution")
		}
	})
}
