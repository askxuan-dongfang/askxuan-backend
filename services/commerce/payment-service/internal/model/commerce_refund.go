package model

import (
	"context"
	"encoding/json"
	"github.com/askxuan/common"
	"github.com/askxuan/common/mqoutbox"
	"github.com/askxuan/payment-service/internal/balance"
	"github.com/askxuan/payment-service/internal/points"
	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"math"
	"strings"
)

// AtomicMockRefund commits the refund, payment state and points reversal together.
// One refund per payment is supported; identical retries return the original record.
func AtomicMockRefund(ctx context.Context, db sqlx.SqlConn, paymentID int64, amount float64, reason string) (*Refund, error) {
	if math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 || len([]rune(reason)) > 255 {
		return nil, common.ErrParam
	}
	if _, err := balance.Cents(amount); err != nil {
		return nil, err
	}
	amount = math.Round(amount*100) / 100
	if amount <= 0 {
		return nil, common.ErrParam
	}
	var result Refund
	err := db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var p struct {
			Status    string  `db:"status"`
			Amount    float64 `db:"amount"`
			Channel   string  `db:"channel"`
			User      string  `db:"user_id"`
			No        string  `db:"payment_no"`
			OrderType string  `db:"order_type"`
			OrderNo   string  `db:"order_no"`
		}
		if err := tx.QueryRowCtx(ctx, &p, `SELECT status,amount,channel,user_id,payment_no,order_type,order_no FROM payment WHERE id=? FOR UPDATE`, paymentID); err != nil {
			return err
		}
		if p.Channel != "mock" && p.Channel != "balance" && p.Channel != "demo_balance" {
			return balance.ErrUnavailable
		}
		if p.Status == PaymentStatusRefunded {
			if err := tx.QueryRowCtx(ctx, &result, `SELECT id,refund_no,payment_id,amount,reason,status,create_time FROM refund WHERE payment_id=? AND status='success' ORDER BY id DESC LIMIT 1`, paymentID); err != nil {
				return err
			}
			if math.Abs(result.Amount-amount) > 0.005 {
				return common.ErrStatusInvalid
			}
			return nil
		}
		if p.Status != PaymentStatusSuccess {
			return common.ErrStatusInvalid
		}
		if (p.Channel == "balance" || p.Channel == "demo_balance") && amount != p.Amount {
			return common.NewBizError(40905, "余额订单当前仅支持整单退款")
		}
		if amount > p.Amount {
			return common.ErrParam
		}
		if p.Channel == "demo_balance" && (p.OrderType == "booking" || p.OrderType == "consultation") {
			var settled int64
			if err := tx.QueryRowCtx(ctx, &settled, `SELECT COUNT(*) FROM askxuan_finance.demo_settlement WHERE source_type=? AND source_no=?`, p.OrderType, p.OrderNo); err != nil {
				return err
			}
			if settled > 0 {
				return common.NewBizError(40907, "服务已经结算，不能直接退回测试余额")
			}
		}
		result = Refund{PaymentId: paymentID, RefundNo: "RF" + strings.ReplaceAll(uuid.NewString(), "-", "")[:30], Amount: amount, Reason: reason, Status: RefundStatusSuccess}
		res, err := tx.ExecCtx(ctx, `INSERT INTO refund(refund_no,payment_id,amount,reason,status,create_time) VALUES(?,?,?,?,?,NOW())`, result.RefundNo, paymentID, amount, reason, RefundStatusSuccess)
		if err != nil {
			return err
		}
		result.Id, err = res.LastInsertId()
		if err != nil {
			return err
		}
		if p.Channel == "balance" {
			if err := balance.Change(ctx, tx, p.User, "refund:"+result.RefundNo, "order_refund", p.No, int64(math.Round(amount*100)), 0); err != nil {
				return err
			}
		}
		if p.Channel == "demo_balance" {
			if err := balance.ChangeDemo(ctx, tx, p.User, "refund:"+result.RefundNo, "order_refund", p.No, int64(math.Round(amount*100)), 0); err != nil {
				return err
			}
		}
		if err := points.Reverse(ctx, tx, paymentID, result.Id); err != nil {
			return err
		}
		_, err = tx.ExecCtx(ctx, `UPDATE payment SET status='refunded' WHERE id=?`, paymentID)
		if err != nil {
			return err
		}
		body, _ := json.Marshal(map[string]any{"paymentNo": p.No, "userId": p.User, "orderType": p.OrderType, "orderNo": p.OrderNo, "amount": p.Amount, "action": "refunded", "channel": p.Channel})
		return mqoutbox.Enqueue(ctx, tx, "payment:"+p.No+":refunded", "payment", p.No, "payment.refunded", "payment.events", "", string(body))
	})
	return &result, err
}
