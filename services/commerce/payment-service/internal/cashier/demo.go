package cashier

import (
	"context"
	"github.com/askxuan/common"
	"github.com/askxuan/payment-service/internal/balance"
	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"strings"
)

const demoRechargeSelect = `SELECT recharge_no,user_id,request_id,channel,amount_cents,status,COALESCE(trade_no,'') trade_no,pay_url,refund_no,refund_cents,DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s') created_at FROM demo_wallet_recharge`

// DemoRecharge uses no gateway and never credits cash or reward points.
func (s *Store) DemoRecharge(ctx context.Context, user, request string, cents int64) (Recharge, error) {
	var f Recharge
	if !s.DemoEnabled || !s.Mock || s.Enabled {
		return f, balance.ErrUnavailable
	}
	if user == "" || cents < 100 || cents > 500000 || len(request) < 16 || len(request) > 64 {
		return f, common.ErrParam
	}
	err := s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		no := "D" + strings.ReplaceAll(uuid.NewString(), "-", "")[:30]
		if _, e := tx.ExecCtx(ctx, `INSERT INTO demo_wallet_recharge(recharge_no,user_id,request_id,channel,amount_cents,pay_url) VALUES(?,?,?,'demo',?,'') ON DUPLICATE KEY UPDATE recharge_no=recharge_no`, no, user, request, cents); e != nil {
			return e
		}
		if e := tx.QueryRowCtx(ctx, &f, demoRechargeSelect+` WHERE user_id=? AND request_id=? FOR UPDATE`, user, request); e != nil {
			return e
		}
		if f.Cents != cents {
			return balance.ErrConflict
		}
		if f.Status != "pending" {
			return nil
		}
		if e := balance.ChangeDemo(ctx, tx, user, "recharge:"+f.No, "recharge", f.No, cents, 0); e != nil {
			return e
		}
		_, e := tx.ExecCtx(ctx, `UPDATE demo_wallet_recharge SET status='success',trade_no=? WHERE recharge_no=?`, "DEMO-"+f.No, f.No)
		f.Status = "success"
		f.Trade = "DEMO-" + f.No
		return e
	})
	return f, err
}
func (s *Store) demoGet(ctx context.Context, user, no string) (Recharge, error) {
	var f Recharge
	e := s.DB.QueryRowCtx(ctx, &f, demoRechargeSelect+` WHERE recharge_no=? AND user_id=?`, no, user)
	if e == sqlx.ErrNotFound {
		return f, common.ErrForbidden
	}
	return f, e
}
func (s *Store) demoRefund(ctx context.Context, user, no string, cents int64) (Recharge, error) {
	var f Recharge
	if !s.DemoEnabled || !s.Mock || s.Enabled {
		return f, balance.ErrUnavailable
	}
	if cents <= 0 || cents > 500000 {
		return f, common.ErrParam
	}
	e := s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		if e := tx.QueryRowCtx(ctx, &f, demoRechargeSelect+` WHERE recharge_no=? AND user_id=? FOR UPDATE`, no, user); e != nil {
			if e == sqlx.ErrNotFound {
				return common.ErrForbidden
			}
			return e
		}
		if f.Status == "refunded" && f.RefundCents == cents {
			return nil
		}
		if f.Status != "success" || cents > f.Cents {
			return balance.ErrConflict
		}
		if e := balance.ChangeDemo(ctx, tx, user, "recharge-refund:"+no, "recharge_refund", no, -cents, 0); e != nil {
			return e
		}
		f.RefundNo = "DR" + strings.ReplaceAll(uuid.NewString(), "-", "")[:29]
		f.RefundCents = cents
		f.Status = "refunded"
		_, e := tx.ExecCtx(ctx, `UPDATE demo_wallet_recharge SET status='refunded',refund_no=?,refund_cents=? WHERE recharge_no=?`, f.RefundNo, cents, no)
		return e
	})
	return f, e
}
