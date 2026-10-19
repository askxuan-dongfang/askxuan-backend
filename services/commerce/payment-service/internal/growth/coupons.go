package growth

import (
	"context"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type Budget struct {
	Kind     string `db:"kind" json:"kind"`
	Total    int64  `db:"total_cents" json:"totalCents"`
	Held     int64  `db:"held_cents" json:"heldCents"`
	Spent    int64  `db:"spent_cents" json:"spentCents"`
	Revision int64  `db:"revision" json:"revision"`
}

func lockBudget(ctx context.Context, tx sqlx.Session, kind string) (b Budget, e error) {
	e = tx.QueryRowCtx(ctx, &b, "SELECT kind,total_cents,held_cents,spent_cents,revision FROM growth_budget WHERE kind=? FOR UPDATE", kind)
	return
}

// The full face value is funded at issue time. An unfunded gift is explicitly
// pending, never a spendable promise. Account lock precedes both budget locks.
func (s *Store) syncCoupons(ctx context.Context, tx sqlx.Session, a Account) error {
	b, e := lockBudget(ctx, tx, "coupon")
	if e != nil {
		return e
	}
	now := s.now().Unix()
	tier := Tier(a.Total)
	coupons := []Coupon{}
	e = tx.QueryRowsCtx(ctx, &coupons, "SELECT id,level,minimum_cents,value_cents,status,expires_at FROM growth_coupon WHERE user_id=? ORDER BY level FOR UPDATE", a.User)
	if e != nil {
		return e
	}
	seen := map[int]bool{}
	for _, c := range coupons {
		seen[c.Level] = true
		status := c.Status
		if c.Status == "unused" {
			if c.Level > tier.Code {
				status = "revoked"
			} else if c.Expires <= now {
				status = "expired"
			}
			if status != c.Status {
				b.Held -= c.Value
			}
		}
		if c.Status == "pending_budget" && c.Level > tier.Code {
			status = "revoked"
		}
		if status != c.Status {
			if _, e = tx.ExecCtx(ctx, "UPDATE growth_coupon SET status=? WHERE id=?", status, c.ID); e != nil {
				return e
			}
		}
		if status == "pending_budget" && b.Total-b.Held-b.Spent >= c.Value {
			b.Held += c.Value
			if _, e = tx.ExecCtx(ctx, "UPDATE growth_coupon SET status='unused',issued_at=?,expires_at=? WHERE id=?", now, now+30*86400, c.ID); e != nil {
				return e
			}
		}
	}
	for _, level := range Levels() {
		if level.Code < 2 || level.Code > tier.Code || seen[level.Code] {
			continue
		}
		status := "pending_budget"
		issued, expires := int64(0), int64(0)
		if b.Total-b.Held-b.Spent >= level.CouponValue {
			status = "unused"
			issued = now
			expires = now + 30*86400
			b.Held += level.CouponValue
		}
		_, e = tx.ExecCtx(ctx, "INSERT INTO growth_coupon(user_id,level,minimum_cents,value_cents,status,issued_at,expires_at) VALUES(?,?,?,?,?,?,?)", a.User, level.Code, level.CouponMinimum, level.CouponValue, status, issued, expires)
		if e != nil {
			return e
		}
	}
	_, e = tx.ExecCtx(ctx, "UPDATE growth_budget SET held_cents=? WHERE kind='coupon'", b.Held)
	return e
}
