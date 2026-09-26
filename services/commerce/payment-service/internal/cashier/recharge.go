package cashier

import (
	"context"
	"errors"
	"github.com/askxuan/common"
	"github.com/askxuan/payment-service/internal/balance"
	"github.com/askxuan/payment-service/internal/paychannel"
	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"strings"
	"time"
)

type Store struct {
	DB            sqlx.SqlConn
	Enabled, Mock bool
	Channels      map[string]paychannel.Gateway
}
type Recharge struct {
	No          string `db:"recharge_no" json:"rechargeNo"`
	User        string `db:"user_id" json:"-"`
	Request     string `db:"request_id" json:"-"`
	Channel     string `db:"channel" json:"channel"`
	Cents       int64  `db:"amount_cents" json:"amountCents"`
	Status      string `db:"status" json:"status"`
	Trade       string `db:"trade_no" json:"tradeNo"`
	URL         string `db:"pay_url" json:"payUrl"`
	RefundNo    string `db:"refund_no" json:"refundNo"`
	RefundCents int64  `db:"refund_cents" json:"refundCents"`
	Created     string `db:"created_at" json:"createdAt"`
}

const rechargeSelect = `SELECT recharge_no,user_id,request_id,channel,amount_cents,status,COALESCE(trade_no,'') trade_no,pay_url,refund_no,refund_cents,DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s') created_at FROM wallet_recharge`

func (s *Store) CreateRecharge(ctx context.Context, user, channel, request, ip string, cents int64) (Recharge, error) {
	var f Recharge
	g := s.Channels[channel]
	if !s.Enabled || g == nil {
		return f, balance.ErrUnavailable
	}
	if cents < 100 || cents > 500000 || len(request) < 16 || len(request) > 64 {
		return f, common.ErrParam
	}
	no := "W" + strings.ReplaceAll(uuid.NewString(), "-", "")[:30]
	// Stable client request ID preserves the same merchant order after network errors.
	_, e := s.DB.ExecCtx(ctx, `INSERT INTO wallet_recharge(recharge_no,user_id,request_id,channel,amount_cents,pay_url) VALUES(?,?,?,?,?,'') ON DUPLICATE KEY UPDATE recharge_no=recharge_no`, no, user, request, channel, cents)
	if e != nil {
		return f, e
	}
	e = s.DB.QueryRowCtx(ctx, &f, rechargeSelect+` WHERE user_id=? AND request_id=?`, user, request)
	if e != nil {
		return f, e
	}
	if f.Channel != channel || f.Cents != cents {
		return f, balance.ErrConflict
	}
	if f.Status != "pending" {
		f.URL = ""
		return f, nil
	}
	if f.URL == "" {
		f.URL, e = g.Checkout(ctx, paychannel.Order{No: f.No, Cents: cents, IP: ip})
		if e != nil {
			return f, e
		}
		// Signed checkout URLs are returned once, never stored in SQL/logs.
	}
	return f, e
}
func (s *Store) Apply(ctx context.Context, channel string, r paychannel.Result) error {
	if r.State != "success" || r.TradeNo == "" || r.Cents <= 0 {
		return paychannel.ErrVerification
	}
	return s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var f Recharge
		if e := tx.QueryRowCtx(ctx, &f, rechargeSelect+` WHERE recharge_no=? FOR UPDATE`, r.No); e != nil {
			return e
		}
		if f.Channel != channel || f.Cents != r.Cents {
			return paychannel.ErrVerification
		}
		if f.Status != "pending" {
			if f.Trade == r.TradeNo && (f.Status == "success" || f.Status == "refund_pending" || f.Status == "refunded") {
				return nil
			}
			return balance.ErrConflict
		}
		if e := balance.Change(ctx, tx, f.User, "recharge:"+f.No, "recharge", f.No, f.Cents, 0); e != nil {
			return e
		}
		_, e := tx.ExecCtx(ctx, `UPDATE wallet_recharge SET status='success',trade_no=?,pay_url='' WHERE recharge_no=?`, r.TradeNo, f.No)
		return e
	})
}
func (s *Store) Get(ctx context.Context, user, no string) (Recharge, error) {
	var f Recharge
	e := s.DB.QueryRowCtx(ctx, &f, rechargeSelect+` WHERE recharge_no=? AND user_id=?`, no, user)
	if errors.Is(e, sqlx.ErrNotFound) {
		return f, common.ErrForbidden
	}
	if e != nil {
		return f, e
	}
	if f.Status != "pending" {
		f.URL = ""
	}
	return f, nil
}
func (s *Store) RefundRecharge(ctx context.Context, user, no string, cents int64) (Recharge, error) {
	if !s.Enabled {
		return Recharge{}, balance.ErrUnavailable
	}
	e := s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var f Recharge
		if e := tx.QueryRowCtx(ctx, &f, rechargeSelect+` WHERE recharge_no=? AND user_id=? FOR UPDATE`, no, user); e != nil {
			return common.ErrForbidden
		}
		if f.Status == "refund_pending" || f.Status == "refunded" {
			if f.RefundCents != cents {
				return balance.ErrConflict
			}
			return nil
		}
		if s.Channels[f.Channel] == nil {
			return balance.ErrUnavailable
		}
		if f.Status != "success" || cents <= 0 || cents > f.Cents {
			return balance.ErrConflict
		}
		refund := "R" + strings.ReplaceAll(uuid.NewString(), "-", "")[:30]
		if e := balance.Change(ctx, tx, user, "hold:"+refund, "refund_hold", f.No, -cents, cents); e != nil {
			return e
		}
		_, e := tx.ExecCtx(ctx, `UPDATE wallet_recharge SET status='refund_pending',refund_no=?,refund_cents=?,next_check_at=NOW() WHERE recharge_no=?`, refund, cents, no)
		return e
	})
	if e != nil {
		return Recharge{}, e
	}
	return s.Get(ctx, user, no)
}
func (s *Store) reconcileOne(ctx context.Context, f Recharge) error {
	g := s.Channels[f.Channel]
	if g == nil {
		return nil
	}
	if f.Status == "pending" {
		r, e := g.Query(ctx, f.No)
		if e != nil {
			return e
		}
		if r.No != f.No || r.Cents != f.Cents {
			return paychannel.ErrVerification
		}
		if r.State == "success" {
			return s.Apply(ctx, f.Channel, r)
		}
		if r.State == "closed" {
			_, e = s.DB.ExecCtx(ctx, `UPDATE wallet_recharge SET status='closed',pay_url='' WHERE recharge_no=? AND status='pending'`, f.No)
		}
		return e
	}
	if f.Status != "refund_pending" {
		return nil
	}
	state, e := g.Refund(ctx, paychannel.Refund{No: f.No, TradeNo: f.Trade, RefundNo: f.RefundNo, Cents: f.RefundCents, Total: f.Cents})
	if e != nil {
		return e
	}
	if state != "success" {
		return nil
	}
	return s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var current Recharge
		if e := tx.QueryRowCtx(ctx, &current, rechargeSelect+` WHERE recharge_no=? FOR UPDATE`, f.No); e != nil {
			return e
		}
		if current.Status == "refunded" {
			return nil
		}
		if current.Status != "refund_pending" || current.RefundNo != f.RefundNo {
			return balance.ErrConflict
		}
		if e := balance.Change(ctx, tx, f.User, "refund:"+f.RefundNo, "recharge_refund", f.No, 0, -f.RefundCents); e != nil {
			return e
		}
		_, e := tx.ExecCtx(ctx, `UPDATE wallet_recharge SET status='refunded' WHERE recharge_no=?`, f.No)
		return e
	})
}

// Reconcile makes an ambiguous payment/refund durable. It never releases frozen
// money merely because an HTTP request timed out. Failed verification is logged
// without raw messages, card/account details, key material or signed checkout URLs.
func (s *Store) Reconcile(ctx context.Context) {
	var rows []Recharge
	if e := s.DB.QueryRowsCtx(ctx, &rows, rechargeSelect+` WHERE status IN ('pending','refund_pending') AND next_check_at<=NOW() ORDER BY next_check_at LIMIT 30`); e != nil {
		logx.Error("wallet reconciliation query failed")
		return
	}
	for _, f := range rows {
		if ctx.Err() != nil {
			return
		}
		res, e := s.DB.ExecCtx(ctx, `UPDATE wallet_recharge SET next_check_at=DATE_ADD(NOW(),INTERVAL 60 SECOND) WHERE recharge_no=? AND next_check_at<=NOW()`, f.No)
		if e != nil {
			continue
		}
		n, _ := res.RowsAffected()
		if n != 1 {
			continue
		}
		if e = s.reconcileOne(ctx, f); e != nil {
			logx.Errorf("wallet reconciliation pending: %s", f.No)
		}
	}
}
func (s *Store) Start(ctx context.Context) {
	if !s.Enabled {
		return
	}
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.Reconcile(ctx)
			}
		}
	}()
}
