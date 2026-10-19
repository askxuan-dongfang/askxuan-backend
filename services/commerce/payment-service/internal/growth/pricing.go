package growth

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"strings"
)

type Snapshot struct {
	RuleVersion      string  `json:"ruleVersion"`
	SettingsRevision int64   `json:"settingsRevision"`
	Level            Level   `json:"level"`
	Offers           []Offer `json:"offers"`
	Selected         Offer   `json:"selected"`
	OrderExpires     int64   `json:"orderExpiresAt"`
}
type PriceQuote struct {
	ID           string   `db:"quote_id" json:"quoteId"`
	User         string   `db:"user_id" json:"-"`
	Kind         string   `db:"order_type" json:"orderType"`
	No           string   `db:"order_no" json:"orderNo"`
	Root         string   `db:"root_checkout_id" json:"rootCheckoutId"`
	Version      string   `db:"rule_version" json:"ruleVersion"`
	Level        int      `db:"level" json:"levelCode"`
	Status       string   `db:"status" json:"status"`
	SelectedKind string   `db:"selected_kind" json:"selectedKind"`
	CouponID     int64    `db:"coupon_id" json:"couponId"`
	Gross        int64    `db:"gross_cents" json:"grossCents"`
	Discount     int64    `db:"discount_cents" json:"discountCents"`
	Payable      int64    `db:"payable_cents" json:"payableCents"`
	GrowthBase   int64    `db:"growth_base_cents" json:"growthBaseCents"`
	GoodsNet     int64    `db:"goods_net_cents" json:"goodsNetCents"`
	Month        string   `db:"business_month" json:"businessMonth"`
	Raw          string   `db:"snapshot_json" json:"-"`
	Expires      int64    `db:"expires_at" json:"expiresAt"`
	Created      int64    `db:"created_at" json:"createdAt"`
	Payment      string   `db:"payment_no" json:"paymentNo"`
	Refunded     int64    `db:"refunded_cents" json:"refundedCents"`
	Snapshot     Snapshot `db:"-" json:"snapshot"`
}

const quoteFields = "quote_id,user_id,order_type,order_no,root_checkout_id,rule_version,level,status,selected_kind,coupon_id,gross_cents,discount_cents,payable_cents,growth_base_cents,goods_net_cents,business_month,snapshot_json,expires_at,created_at,payment_no,refunded_cents"

func readQuote(ctx context.Context, q reader, id, user string, lock bool) (v PriceQuote, e error) {
	query := "SELECT " + quoteFields + " FROM growth_quote WHERE quote_id=? AND user_id=?"
	if lock {
		query += " FOR UPDATE"
	}
	e = q.QueryRowPartialCtx(ctx, &v, query, id, user)
	if e == nil {
		e = json.Unmarshal([]byte(v.Raw), &v.Snapshot)
	}
	return
}
func (s *Store) candidates(ctx context.Context, tx sqlx.Session, a Account, kind, no string, gross int64) (snap Snapshot, e error) {
	var cfg Settings
	e = tx.QueryRowCtx(ctx, &cfg, "SELECT enabled,discounts_enabled,revision,rule_version,started_at FROM growth_settings WHERE id=1")
	if e != nil {
		return
	}
	if !cfg.Enabled {
		return snap, ErrUnavailable
	}
	snap.RuleVersion = cfg.Version
	snap.SettingsRevision = cfg.Revision
	snap.Level = Tier(a.Total)
	lines, expiry, e := sourceLines(ctx, tx, kind, no, gross)
	if e != nil {
		return snap, e
	}
	snap.OrderExpires = expiry
	lines, e = applyPolicies(ctx, tx, lines, cfg.Discounts, snap.Level)
	if e != nil {
		return snap, e
	}
	var usage struct {
		Held  int64 `db:"held_cents"`
		Spent int64 `db:"spent_cents"`
	}
	e = tx.QueryRowCtx(ctx, &usage, "SELECT held_cents,spent_cents FROM growth_month_usage WHERE user_id=? AND business_month=?", a.User, BusinessMonth(s.now()))
	if e != nil && !errors.Is(e, sqlx.ErrNotFound) {
		return snap, e
	}
	coupons := []Coupon{}
	if cfg.Discounts {
		e = tx.QueryRowsCtx(ctx, &coupons, "SELECT id,level,minimum_cents,value_cents,status,expires_at FROM growth_coupon WHERE user_id=? AND status='unused' AND level<=? AND expires_at>?", a.User, snap.Level.Code, s.now().Unix())
		if e != nil {
			return snap, e
		}
	}
	offers, e := Candidates(lines, snap.Level, MonthlyDiscountLimit-usage.Held-usage.Spent, coupons)
	if e != nil {
		return snap, e
	}
	var budget Budget
	e = tx.QueryRowCtx(ctx, &budget, "SELECT kind,total_cents,held_cents,spent_cents,revision FROM growth_budget WHERE kind='level'")
	if e != nil {
		return snap, e
	}
	snap.Offers = []Offer{}
	for _, o := range offers {
		valid := true
		if o.Kind == "level" && (o.Discount == 0 || o.Discount > budget.Total-budget.Held-budget.Spent) {
			valid = false
		}
		for _, l := range o.Lines {
			if l.Discount > 0 && (o.Kind == "level" || o.Kind == "coupon") && l.Discount > l.MaxDiscount {
				valid = false
			}
		}
		if valid {
			snap.Offers = append(snap.Offers, o)
		}
	}
	if len(snap.Offers) == 0 {
		return snap, ErrChanged
	}
	snap.Selected = snap.Offers[0]
	return snap, nil
}
func setOffer(q *PriceQuote, o Offer) {
	q.SelectedKind = o.Kind
	q.CouponID = o.CouponID
	q.Payable = o.Payable
	q.Discount = o.Discount
	q.GrowthBase = 0
	q.GoodsNet = 0
	for _, l := range o.Lines {
		if l.GrowthEligible {
			q.GrowthBase += l.Net
		}
		if l.Category == "goods" {
			q.GoodsNet += l.Net
		}
	}
	q.Snapshot.Selected = o
	raw, _ := json.Marshal(q.Snapshot)
	q.Raw = string(raw)
}
func insertQuote(ctx context.Context, tx sqlx.Session, q PriceQuote) error {
	_, e := tx.ExecCtx(ctx, "INSERT INTO growth_quote(quote_id,user_id,order_type,order_no,root_checkout_id,rule_version,level,status,selected_kind,coupon_id,gross_cents,discount_cents,payable_cents,growth_base_cents,goods_net_cents,business_month,snapshot_json,expires_at,created_at,payment_no) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)", q.ID, q.User, q.Kind, q.No, q.Root, q.Version, q.Level, q.Status, q.SelectedKind, q.CouponID, q.Gross, q.Discount, q.Payable, q.GrowthBase, q.GoodsNet, q.Month, q.Raw, q.Expires, q.Created, q.Payment)
	return e
}

// PreviewTx runs after the business order is locked and ownership validated.
func (s *Store) PreviewTx(ctx context.Context, tx sqlx.Session, user, kind, no string, gross int64) (v PriceQuote, e error) {
	a, e := lockAccount(ctx, tx, user, s.now().Unix())
	if e != nil {
		return v, e
	}
	var count int64
	e = tx.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM growth_quote WHERE user_id=? AND created_at>?", user, s.now().Unix()-60)
	if e != nil {
		return v, e
	}
	if count >= 20 {
		return v, ErrLimit
	}
	if _, e = lockBudget(ctx, tx, "coupon"); e != nil {
		return v, e
	}
	old := []PriceQuote{}
	e = tx.QueryRowsPartialCtx(ctx, &old, "SELECT "+quoteFields+" FROM growth_quote WHERE user_id=? AND order_type=? AND order_no=? AND status='held' FOR UPDATE", user, kind, no)
	if e != nil {
		return v, e
	}
	for _, q := range old {
		if e = s.release(ctx, tx, a, &q); e != nil {
			return v, e
		}
	}
	if e = s.syncCoupons(ctx, tx, a); e != nil {
		return v, e
	}
	snap, e := s.candidates(ctx, tx, a, kind, no, gross)
	if e != nil {
		return v, e
	}
	now := s.now().Unix()
	v = PriceQuote{ID: strings.ReplaceAll(uuid.NewString(), "-", ""), User: user, Kind: kind, No: no, Root: kind + ":" + no, Version: RuleVersion, Level: snap.Level.Code, Status: "preview", Gross: gross, Month: BusinessMonth(s.now()), Expires: now + 300, Created: now, Snapshot: snap}
	if snap.OrderExpires > 0 && snap.OrderExpires < v.Expires {
		v.Expires = snap.OrderExpires
	}
	setOffer(&v, snap.Selected)
	e = insertQuote(ctx, tx, v)
	return
}
func (s *Store) ReserveTx(ctx context.Context, tx sqlx.Session, user, kind, no, id, selected string, couponID, expected, gross int64) (v PriceQuote, e error) {
	a, e := lockAccount(ctx, tx, user, s.now().Unix())
	if e != nil {
		return v, e
	}
	v, e = readQuote(ctx, tx, id, user, true)
	if e != nil {
		return v, e
	}
	if v.Kind != kind || v.No != no || v.Gross != gross || v.Expires <= s.now().Unix() {
		return v, ErrChanged
	}
	if v.Status == "held" {
		if v.SelectedKind != selected || v.CouponID != couponID || v.Payable != expected {
			return v, ErrChanged
		}
		return v, nil
	}
	if v.Status != "preview" {
		return v, ErrChanged
	}
	// A second preview cannot reserve another coupon/budget for the same checkout.
	var n int64
	e = tx.QueryRowCtx(ctx, &n, "SELECT COUNT(*) FROM growth_quote WHERE order_type=? AND order_no=? AND status='held'", kind, no)
	if e != nil {
		return v, e
	}
	if n > 0 {
		return v, ErrChanged
	}
	fresh, e := s.candidates(ctx, tx, a, kind, no, gross)
	if e != nil {
		return v, e
	}
	if fresh.SettingsRevision != v.Snapshot.SettingsRevision || fresh.Level.Code != v.Level {
		return v, ErrChanged
	}
	var chosen, original *Offer
	for i := range fresh.Offers {
		o := &fresh.Offers[i]
		if o.Kind == selected && o.CouponID == couponID && o.Payable == expected {
			chosen = o
		}
	}
	for i := range v.Snapshot.Offers {
		o := &v.Snapshot.Offers[i]
		if o.Kind == selected && o.CouponID == couponID && o.Payable == expected {
			original = o
		}
	}
	if chosen == nil || original == nil {
		return v, ErrChanged
	}
	newBytes, _ := json.Marshal(chosen)
	oldBytes, _ := json.Marshal(original)
	if string(newBytes) != string(oldBytes) {
		return v, ErrChanged
	}
	if chosen.Kind == "level" && chosen.Discount > 0 {
		b, err := lockBudget(ctx, tx, "level")
		if err != nil {
			return v, err
		}
		if b.Total-b.Held-b.Spent < chosen.Discount {
			return v, ErrChanged
		}
		if _, e = tx.ExecCtx(ctx, "INSERT IGNORE INTO growth_month_usage(user_id,business_month) VALUES(?,?)", user, BusinessMonth(s.now())); e != nil {
			return v, e
		}
		res, err := tx.ExecCtx(ctx, "UPDATE growth_month_usage SET held_cents=held_cents+? WHERE user_id=? AND business_month=? AND held_cents+spent_cents+?<=?", chosen.Discount, user, BusinessMonth(s.now()), chosen.Discount, MonthlyDiscountLimit)
		if err != nil {
			return v, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return v, err
		}
		if n != 1 {
			return v, ErrChanged
		}
		_, e = tx.ExecCtx(ctx, "UPDATE growth_budget SET held_cents=held_cents+? WHERE kind='level'", chosen.Discount)
		if e != nil {
			return v, e
		}
	}
	if chosen.Kind == "coupon" {
		res, err := tx.ExecCtx(ctx, "UPDATE growth_coupon SET status='reserved',quote_id=? WHERE id=? AND user_id=? AND status='unused' AND expires_at>?", v.ID, couponID, user, s.now().Unix())
		if err != nil {
			return v, err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return v, err
		}
		if n != 1 {
			return v, ErrChanged
		}
	}
	v.Status = "held"
	v.Month = BusinessMonth(s.now())
	v.Expires = s.now().Unix() + 900
	if fresh.OrderExpires > 0 && fresh.OrderExpires < v.Expires {
		v.Expires = fresh.OrderExpires
	}
	v.Snapshot = fresh
	setOffer(&v, *chosen)
	_, e = tx.ExecCtx(ctx, "UPDATE growth_quote SET status='held',selected_kind=?,coupon_id=?,discount_cents=?,payable_cents=?,growth_base_cents=?,goods_net_cents=?,business_month=?,snapshot_json=?,expires_at=? WHERE quote_id=?", v.SelectedKind, v.CouponID, v.Discount, v.Payable, v.GrowthBase, v.GoodsNet, v.Month, v.Raw, v.Expires, v.ID)
	return
}
func (s *Store) release(ctx context.Context, tx sqlx.Session, a Account, q *PriceQuote) error {
	if q.Status != "held" {
		return nil
	}
	if q.SelectedKind == "level" && q.Discount > 0 {
		if _, e := lockBudget(ctx, tx, "level"); e != nil {
			return e
		}
		if _, e := tx.ExecCtx(ctx, "UPDATE growth_budget SET held_cents=held_cents-? WHERE kind='level'", q.Discount); e != nil {
			return e
		}
		if _, e := tx.ExecCtx(ctx, "UPDATE growth_month_usage SET held_cents=held_cents-? WHERE user_id=? AND business_month=?", q.Discount, q.User, q.Month); e != nil {
			return e
		}
	}
	if q.CouponID > 0 {
		if _, e := tx.ExecCtx(ctx, "UPDATE growth_coupon SET status='unused',quote_id='' WHERE id=? AND quote_id=? AND status='reserved'", q.CouponID, q.ID); e != nil {
			return e
		}
		if e := s.syncCoupons(ctx, tx, a); e != nil {
			return e
		}
	}
	_, e := tx.ExecCtx(ctx, "UPDATE growth_quote SET status='released' WHERE quote_id=? AND status='held'", q.ID)
	q.Status = "released"
	return e
}

// PayableTx validates an immutable, reserved quote. The account lock serializes
// expiry, refunds and simultaneous checkout operations for this customer.
func (s *Store) PayableTx(ctx context.Context, tx sqlx.Session, user, kind, no, id string, gross, expected int64) (q PriceQuote, e error) {
	_, e = lockAccount(ctx, tx, user, s.now().Unix())
	if e != nil {
		return
	}
	q, e = readQuote(ctx, tx, id, user, true)
	if e != nil {
		return
	}
	if q.Kind != kind || q.No != no || q.Gross != gross || q.Payable != expected || q.Status != "held" || q.Expires <= s.now().Unix() {
		return q, ErrChanged
	}
	return
}
func (s *Store) PaidTx(ctx context.Context, tx sqlx.Session, user, kind, no string, paymentID int64, paymentNo string, gross, paid int64, quoteID string) error {
	// Existing reservations remain payable; disabling growth only stops new accrual.
	if quoteID == "" {
		var enabled bool
		if err := tx.QueryRowCtx(ctx, &enabled, "SELECT enabled FROM growth_settings WHERE id=1"); err != nil {
			return err
		}
		if !enabled {
			return nil
		}
	}
	a, e := lockAccount(ctx, tx, user, s.now().Unix())
	if e != nil {
		return e
	}
	var q PriceQuote
	if quoteID != "" {
		q, e = readQuote(ctx, tx, quoteID, user, true)
		if e != nil {
			return e
		}
		if q.Status != "held" || q.Payable != paid {
			return ErrChanged
		}
		if q.SelectedKind == "level" && q.Discount > 0 {
			if _, e = lockBudget(ctx, tx, "level"); e != nil {
				return e
			}
			if _, e = tx.ExecCtx(ctx, "UPDATE growth_budget SET held_cents=held_cents-?,spent_cents=spent_cents+? WHERE kind='level'", q.Discount, q.Discount); e != nil {
				return e
			}
			if _, e = tx.ExecCtx(ctx, "UPDATE growth_month_usage SET held_cents=held_cents-?,spent_cents=spent_cents+? WHERE user_id=? AND business_month=?", q.Discount, q.Discount, user, q.Month); e != nil {
				return e
			}
		}
		if q.CouponID > 0 {
			if _, e = lockBudget(ctx, tx, "coupon"); e != nil {
				return e
			}
			if _, e = tx.ExecCtx(ctx, "UPDATE growth_budget SET held_cents=held_cents-?,spent_cents=spent_cents+? WHERE kind='coupon'", q.Discount, q.Discount); e != nil {
				return e
			}
			if _, e = tx.ExecCtx(ctx, "UPDATE growth_coupon SET status='used' WHERE id=? AND quote_id=? AND status='reserved'", q.CouponID, q.ID); e != nil {
				return e
			}
		}
		_, e = tx.ExecCtx(ctx, "UPDATE growth_quote SET status='paid',payment_no=? WHERE quote_id=?", paymentNo, q.ID)
		if e != nil {
			return e
		}
	} else {
		lines, _, err := sourceLines(ctx, tx, kind, no, gross)
		if err != nil {
			return err
		}
		offers, err := Candidates(lines, Tier(a.Total), 0, nil)
		if err != nil {
			return err
		}
		normal := offers[0]
		q = PriceQuote{ID: strings.ReplaceAll(uuid.NewString(), "-", ""), User: user, Kind: kind, No: no, Root: kind + ":" + no, Version: RuleVersion, Level: Tier(a.Total).Code, Status: "paid", Gross: gross, Month: BusinessMonth(s.now()), Created: s.now().Unix(), Payment: paymentNo, Snapshot: Snapshot{RuleVersion: RuleVersion, Level: Tier(a.Total), Offers: []Offer{normal}, Selected: normal}}
		setOffer(&q, normal)
		if e = insertQuote(ctx, tx, q); e != nil {
			return e
		}
	}
	_, e = tx.ExecCtx(ctx, "INSERT INTO growth_transaction(payment_id,user_id,order_type,order_no,gross_cents,paid_cents,growth_base_cents,quote_id,next_check_at,created_at) VALUES(?,?,?,?,?,?,?,?,?,?)", paymentID, user, kind, no, gross, paid, q.GrowthBase, q.ID, s.now().Unix(), s.now().Unix())
	return e
}
func (s *Store) OrderSnapshot(ctx context.Context, user, kind, no string) (v *PriceQuote, e error) {
	var id string
	e = s.DB.QueryRowCtx(ctx, &id, "SELECT quote_id FROM growth_quote WHERE user_id=? AND order_type=? AND order_no=? AND status IN ('paid','refunded') ORDER BY created_at DESC LIMIT 1", user, kind, no)
	if errors.Is(e, sqlx.ErrNotFound) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	q, e := readQuote(ctx, s.DB, id, user, false)
	return &q, e
}
