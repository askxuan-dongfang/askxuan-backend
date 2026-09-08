package model

import (
	"context"
	"github.com/askxuan/common"
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
	amount = math.Round(amount*100) / 100
	if amount <= 0 {
		return nil, common.ErrParam
	}
	var result Refund
	err := db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var p struct {
			Status string  `db:"status"`
			Amount float64 `db:"amount"`
		}
		if err := tx.QueryRowCtx(ctx, &p, `SELECT status,amount FROM payment WHERE id=? FOR UPDATE`, paymentID); err != nil {
			return err
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
		if amount > p.Amount {
			return common.ErrParam
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
		if err := points.Reverse(ctx, tx, paymentID, result.Id); err != nil {
			return err
		}
		_, err = tx.ExecCtx(ctx, `UPDATE payment SET status='refunded' WHERE id=?`, paymentID)
		return err
	})
	return &result, err
}
