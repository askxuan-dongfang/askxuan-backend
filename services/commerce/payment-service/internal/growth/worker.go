package growth

import (
	"context"
	"errors"
	"fmt"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"time"
)

type Transaction struct {
	PaymentID  int64  `db:"payment_id"`
	User       string `db:"user_id"`
	Kind       string `db:"order_type"`
	No         string `db:"order_no"`
	Gross      int64  `db:"gross_cents"`
	Paid       int64  `db:"paid_cents"`
	Base       int64  `db:"growth_base_cents"`
	QuoteID    string `db:"quote_id"`
	Completion int64  `db:"completion_at"`
	Refund     int64  `db:"refund_cents"`
	Next       int64  `db:"next_check_at"`
	Created    int64  `db:"created_at"`
}

const transactionFields = "payment_id,user_id,order_type,order_no,gross_cents,paid_cents,growth_base_cents,quote_id,completion_at,refund_cents,next_check_at,created_at"

// completionFacts reads existing fulfillment evidence. Customer receipt is
// required for bookings; submitting a provider receipt alone is insufficient.
func completionFacts(ctx context.Context, q reader, t Transaction, now int64) (completed int64, paused bool, e error) {
	var row struct {
		Status string `db:"status"`
		At     int64  `db:"at"`
	}
	query := ""
	switch t.Kind {
	case "shop_order":
		query = "SELECT status,UNIX_TIMESTAMP(update_time) at FROM askxuan_order.shop_order WHERE order_no=?"
	case "diy_order":
		query = "SELECT status,UNIX_TIMESTAMP(update_time) at FROM askxuan_diy.diy_order WHERE order_no=?"
	case "booking":
		query = "SELECT b.status,COALESCE((SELECT MIN(UNIX_TIMESTAMP(l.create_time)) FROM askxuan_booking.booking_status_log l WHERE l.booking_id=b.booking_no AND l.to_status='completed' AND l.operator_type='user'),0) at FROM askxuan_booking.booking b WHERE b.booking_no=?"
	case "consultation":
		query = "SELECT status,COALESCE(UNIX_TIMESTAMP(expires_at),0) at FROM askxuan_booking.consultation_order WHERE order_no=?"
	case "ai_report":
		query = "SELECT status,0 at FROM askxuan_ai.ai_report WHERE report_no=?"
	default:
		return 0, false, ErrChanged
	}
	if e = q.QueryRowCtx(ctx, &row, query, t.No); e != nil {
		return
	}
	paused = row.Status == "in_return" || row.Status == "refund_pending"
	if t.Kind == "shop_order" {
		var active int64
		e = q.QueryRowCtx(ctx, &active, "SELECT COUNT(*) FROM askxuan_order.return_order r JOIN askxuan_order.shop_order o ON o.id=r.order_id WHERE o.order_no=? AND r.status NOT IN ('completed','rejected','cancelled')", t.No)
		if e != nil {
			return
		}
		paused = paused || active > 0
	}
	switch t.Kind {
	case "shop_order", "diy_order", "booking":
		if row.Status == "completed" || row.Status == "reviewed" {
			completed = row.At
		}
	case "consultation":
		if (row.Status == "active" || row.Status == "expired") && row.At > 0 && row.At <= now {
			completed = row.At
		}
	case "ai_report":
		if row.Status == "ready" {
			completed = t.Created
		}
	}
	if completed > now {
		completed = 0
	}
	return
}
func (s *Store) ReconcilePayment(ctx context.Context, id int64) error {
	return s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var payment struct {
			Status  string `db:"status"`
			Channel string `db:"channel"`
		}
		e := tx.QueryRowCtx(ctx, &payment, "SELECT status,channel FROM payment WHERE id=? FOR UPDATE", id)
		if e != nil {
			return e
		}
		var t Transaction
		e = tx.QueryRowCtx(ctx, &t, "SELECT "+transactionFields+" FROM growth_transaction WHERE payment_id=?", id)
		if e != nil {
			return e
		}
		a, e := lockAccount(ctx, tx, t.User, s.now().Unix())
		if e != nil {
			return e
		}
		if payment.Status == "refunded" {
			return s.reverseLocked(ctx, tx, &a, &t)
		}
		if payment.Status != "success" || (payment.Channel != "balance" && payment.Channel != "wechat" && payment.Channel != "alipay") {
			return ErrChanged
		}
		completed, paused, e := completionFacts(ctx, tx, t, s.now().Unix())
		if e != nil {
			return e
		}
		if t.Completion == 0 && completed > 0 {
			t.Completion = completed
		}
		amount := Earned(t.Base)
		state, reason := "pending", "waiting_completion"
		eligible := int64(0)
		if t.Completion > 0 {
			eligible = t.Completion + int64(ObservationPeriod/time.Second)
			reason = "observation_period"
		}
		if paused {
			state = "paused"
			reason = "after_sale"
		}
		if !paused && eligible > 0 && eligible <= s.now().Unix() {
			state = "posted"
			reason = ""
		}
		key := fmt.Sprintf("payment:%d", t.PaymentID)
		var event Event
		e = tx.QueryRowCtx(ctx, &event, "SELECT "+eventFields+" FROM growth_event WHERE event_key=?", key)
		if errors.Is(e, sqlx.ErrNotFound) {
			res, err := tx.ExecCtx(ctx, "INSERT INTO growth_event(event_key,user_id,kind,reference_no,original_award,current_award,state,reason,eligible_at,business_day,business_month,rule_version,created_at,updated_at) VALUES(?,?,'purchase',?,?,?,?,?,?,?,?,?,?,?)", key, t.User, t.No, amount, amount, state, reason, eligible, BusinessDay(s.now()), BusinessMonth(s.now()), RuleVersion, s.now().Unix(), s.now().Unix())
			if err != nil {
				return err
			}
			event.ID, e = res.LastInsertId()
			if e != nil {
				return e
			}
			event.User = t.User
			event.Kind = "purchase"
			event.Reference = t.No
			event.Version = RuleVersion
			event.Current = amount
		} else if e != nil {
			return e
		}
		if state == "posted" && event.State != "revoked" {
			if e = s.applyEvent(ctx, tx, &a, &event, amount, "posted"); e != nil {
				return e
			}
		} else if event.Applied > 0 {
			state = "posted"
			reason = ""
		} // An open case pauses pending growth; only a successful refund reverses posted growth.
		if _, e = tx.ExecCtx(ctx, "UPDATE growth_event SET state=?,reason=?,eligible_at=?,updated_at=? WHERE id=? AND state<>'revoked'", state, reason, eligible, s.now().Unix(), event.ID); e != nil {
			return e
		}
		next := s.now().Unix() + 60
		if state == "posted" {
			next = s.now().Unix() + 3600
		}
		_, e = tx.ExecCtx(ctx, "UPDATE growth_transaction SET completion_at=?,next_check_at=? WHERE payment_id=?", t.Completion, next, t.PaymentID)
		return e
	})
}
func (s *Store) ReverseTx(ctx context.Context, tx sqlx.Session, id int64) error {
	var t Transaction
	e := tx.QueryRowCtx(ctx, &t, "SELECT "+transactionFields+" FROM growth_transaction WHERE payment_id=?", id)
	if errors.Is(e, sqlx.ErrNotFound) {
		return nil
	}
	if e != nil {
		return e
	}
	a, e := lockAccount(ctx, tx, t.User, s.now().Unix())
	if e != nil {
		return e
	}
	return s.reverseLocked(ctx, tx, &a, &t)
}
func (s *Store) reverseLocked(ctx context.Context, tx sqlx.Session, a *Account, t *Transaction) error {
	var refunded int64
	e := tx.QueryRowCtx(ctx, &refunded, "SELECT COALESCE(SUM(CAST(ROUND(amount*100) AS SIGNED)),0) FROM refund WHERE payment_id=? AND status='success'", t.PaymentID)
	if e != nil {
		return e
	}
	if refunded == t.Refund {
		return nil
	}
	if refunded != t.Paid {
		return fmt.Errorf("growth cash refund requires full original payment")
	}
	events := []Event{}
	if e = tx.QueryRowsCtx(ctx, &events, "SELECT "+eventFields+" FROM growth_event WHERE user_id=? AND (event_key=? OR (kind='review' AND reference_no=?))", t.User, fmt.Sprintf("payment:%d", t.PaymentID), t.Kind+":"+t.No); e != nil {
		return e
	}
	for i := range events {
		if e = s.applyEvent(ctx, tx, a, &events[i], 0, "reversed"); e != nil {
			return e
		}
	}
	q, e := readQuote(ctx, tx, t.QuoteID, t.User, true)
	if e != nil {
		return e
	}
	if q.Status == "paid" {
		if q.SelectedKind == "level" && q.Discount > 0 {
			if _, e = lockBudget(ctx, tx, "level"); e != nil {
				return e
			}
			if _, e = tx.ExecCtx(ctx, "UPDATE growth_budget SET spent_cents=spent_cents-? WHERE kind='level'", q.Discount); e != nil {
				return e
			}
			if q.Month == BusinessMonth(s.now()) {
				if _, e = tx.ExecCtx(ctx, "UPDATE growth_month_usage SET spent_cents=spent_cents-? WHERE user_id=? AND business_month=?", q.Discount, q.User, q.Month); e != nil {
					return e
				}
			}
		}
		if q.CouponID > 0 {
			if _, e = lockBudget(ctx, tx, "coupon"); e != nil {
				return e
			}
			var c Coupon
			if e = tx.QueryRowCtx(ctx, &c, "SELECT id,level,minimum_cents,value_cents,status,expires_at FROM growth_coupon WHERE id=? FOR UPDATE", q.CouponID); e != nil {
				return e
			}
			status := "expired"
			held := int64(0)
			if c.Level > Tier(a.Total).Code {
				status = "revoked"
			} else if c.Expires > s.now().Unix() {
				status = "unused"
				held = c.Value
			}
			if _, e = tx.ExecCtx(ctx, "UPDATE growth_budget SET spent_cents=spent_cents-?,held_cents=held_cents+? WHERE kind='coupon'", q.Discount, held); e != nil {
				return e
			}
			if _, e = tx.ExecCtx(ctx, "UPDATE growth_coupon SET status=?,quote_id='' WHERE id=?", status, c.ID); e != nil {
				return e
			}
		}
		if _, e = tx.ExecCtx(ctx, "UPDATE growth_quote SET status='refunded',refunded_cents=? WHERE quote_id=?", refunded, q.ID); e != nil {
			return e
		}
	}
	_, e = tx.ExecCtx(ctx, "UPDATE growth_transaction SET refund_cents=?,next_check_at=? WHERE payment_id=?", refunded, s.now().AddDate(100, 0, 0).Unix(), t.PaymentID)
	return e
}
func (s *Store) reviewScan(ctx context.Context) error {
	type review struct {
		ID        int64 `db:"id"`
		PaymentID int64 `db:"payment_id"`
		Created   int64 `db:"created"`
	}
	rows := []review{}
	// Only new real transactions and verified source ownership. Both historical
	// numeric item IDs and order-number references are normalized by the join.
	e := s.DB.QueryRowsCtx(ctx, &rows, `SELECT r.id,t.payment_id,UNIX_TIMESTAMP(r.create_time) created FROM askxuan_review.review r JOIN growth_transaction t ON t.user_id=r.user_id AND t.order_type=r.target_type LEFT JOIN askxuan_order.shop_order so ON t.order_type='shop_order' AND so.order_no=t.order_no LEFT JOIN askxuan_diy.diy_order d ON t.order_type='diy_order' AND d.order_no=t.order_no LEFT JOIN growth_event e ON e.user_id=t.user_id AND e.kind='review' AND e.reference_no=CONCAT(t.order_type,':',t.order_no) WHERE (r.target_id=t.order_no OR r.target_id=CAST(so.id AS CHAR) OR r.target_id=CAST(d.id AS CHAR)) AND r.status='normal' AND r.create_time>=FROM_UNIXTIME(t.created_at) AND t.refund_cents=0 AND e.id IS NULL ORDER BY r.id LIMIT 100`)
	if e != nil {
		return e
	}
	for _, r := range rows {
		e = s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
			var status string
			if e := tx.QueryRowCtx(ctx, &status, "SELECT status FROM payment WHERE id=? FOR UPDATE", r.PaymentID); e != nil {
				return e
			}
			if status != "success" {
				return nil
			}
			var t Transaction
			if e := tx.QueryRowCtx(ctx, &t, "SELECT "+transactionFields+" FROM growth_transaction WHERE payment_id=?", r.PaymentID); e != nil {
				return e
			}
			a, e := lockAccount(ctx, tx, t.User, s.now().Unix())
			if e != nil {
				return e
			}
			done, paused, e := completionFacts(ctx, tx, t, s.now().Unix())
			if e != nil {
				return e
			}
			if done == 0 || paused {
				return nil
			}
			_, e = s.awardTaskAt(ctx, tx, &a, "review", t.Kind+":"+t.No, 5, 4, "month", time.Unix(r.Created, 0), true)
			return e
		})
		if e != nil {
			return e
		}
	}
	return nil
}
func (s *Store) Tick(ctx context.Context) error {
	cfg, e := s.Settings(ctx)
	if e != nil {
		return e
	}
	quotes := []PriceQuote{}
	if e = s.DB.QueryRowsPartialCtx(ctx, &quotes, "SELECT "+quoteFields+" FROM growth_quote WHERE status='held' AND expires_at<=? ORDER BY expires_at LIMIT 100", s.now().Unix()); e != nil {
		return e
	}
	for _, q := range quotes {
		e = s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
			a, err := lockAccount(ctx, tx, q.User, s.now().Unix())
			if err != nil {
				return err
			}
			current, err := readQuote(ctx, tx, q.ID, q.User, true)
			if err != nil {
				return err
			}
			if current.Status != "held" || current.Expires > s.now().Unix() {
				return nil
			}
			return s.release(ctx, tx, a, &current)
		})
		if e != nil {
			return e
		}
	}
	if !cfg.Enabled {
		return nil
	}
	ids := []int64{}
	if e = s.DB.QueryRowsCtx(ctx, &ids, "SELECT payment_id FROM growth_transaction WHERE next_check_at<=? ORDER BY next_check_at,payment_id LIMIT 100", s.now().Unix()); e != nil {
		return e
	}
	for _, id := range ids {
		if e = s.ReconcilePayment(ctx, id); e != nil {
			return e
		}
	}
	users := []string{}
	if e = s.DB.QueryRowsCtx(ctx, &users, "SELECT DISTINCT user_id FROM growth_coupon WHERE status='pending_budget' OR (status='unused' AND expires_at<=?) LIMIT 100", s.now().Unix()); e != nil {
		return e
	}
	for _, user := range users {
		if e = s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
			a, err := lockAccount(ctx, tx, user, s.now().Unix())
			if err != nil {
				return err
			}
			return s.syncCoupons(ctx, tx, a)
		}); e != nil {
			return e
		}
	}
	if e = s.reviewScan(ctx); e != nil {
		return e
	}
	_, e = s.DB.ExecCtx(ctx, "DELETE FROM growth_quote WHERE status='preview' AND expires_at<? LIMIT 500", s.now().Unix()-86400)
	return e
}
func (s *Store) Start(ctx context.Context) {
	go func() {
		timer := time.NewTicker(time.Minute)
		defer timer.Stop()
		for {
			run, cancel := context.WithTimeout(ctx, 50*time.Second)
			if e := s.Tick(run); e != nil && ctx.Err() == nil {
				logx.Error("growth reconciliation deferred; transaction will retry")
			}
			cancel()
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}
		}
	}()
}
