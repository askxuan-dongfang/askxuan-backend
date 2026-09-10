// Package points owns the independent points ledger and redemption store.
package points

import (
	"context"
	"errors"
	"fmt"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Earned uses integer cents: each payment earns floor(actual yuan / 100).
func Earned(cents int64) int64 {
	if cents <= 0 {
		return 0
	}
	return cents / 10000
}

// Change requires an existing SQL transaction. Lock accounts before changing balances.
// Refund reversals may create debt; further redemptions require a sufficient balance.
func Change(ctx context.Context, tx sqlx.Session, user, key, kind, ref string, delta int64) error {
	if _, err := tx.ExecCtx(ctx, `INSERT INTO points_account(user_id) VALUES(?) ON DUPLICATE KEY UPDATE user_id=user_id`, user); err != nil {
		return err
	}
	var balance int64
	if err := tx.QueryRowCtx(ctx, &balance, `SELECT balance FROM points_account WHERE user_id=? FOR UPDATE`, user); err != nil {
		return err
	}
	var count int64
	if err := tx.QueryRowCtx(ctx, &count, `SELECT COUNT(*) FROM points_ledger WHERE event_key=?`, key); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	if kind == "redeem" && balance+delta < 0 {
		return ErrBalance
	}
	if _, err := tx.ExecCtx(ctx, `UPDATE points_account SET balance=balance+? WHERE user_id=?`, delta, user); err != nil {
		return err
	}
	_, err := tx.ExecCtx(ctx, `INSERT INTO points_ledger(user_id,event_key,kind,delta,balance_after,reference_no) VALUES(?,?,?,?,?,?)`, user, key, kind, delta, balance+delta, ref)
	return err
}

var ErrBalance = errors.New("积分不足")
var ErrConflict = errors.New("商品、库存或订单状态已变化，请刷新后重试")
var ErrInvalid = errors.New("请检查填写的信息")

// Award is called inside the transaction that changes the payment to success.
// Reading DECIMAL amounts as cents in SQL avoids float truncation around thresholds.
func Award(ctx context.Context, tx sqlx.Session, paymentID int64) error {
	var p struct {
		User      string `db:"user_id"`
		No        string `db:"payment_no"`
		Cents     int64  `db:"cents"`
		OrderType string `db:"order_type"`
		OrderNo   string `db:"order_no"`
	}
	if err := tx.QueryRowCtx(ctx, &p, `SELECT user_id,payment_no,order_type,order_no,CAST(ROUND(amount*100) AS SIGNED) cents FROM payment WHERE id=?`, paymentID); err != nil {
		return err
	}
	if p.OrderType == "shop_order" && common.IsExperienceOrder(p.OrderNo) {
		return nil
	}
	var count int64
	if err := tx.QueryRowCtx(ctx, &count, `SELECT COUNT(*) FROM points_payment_award WHERE payment_id=?`, paymentID); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	amount := Earned(p.Cents)
	if _, err := tx.ExecCtx(ctx, `INSERT INTO points_payment_award(payment_id,user_id,awarded) VALUES(?,?,?)`, paymentID, p.User, amount); err != nil {
		return err
	}
	if amount == 0 {
		return nil
	}
	return Change(ctx, tx, p.User, fmt.Sprintf("payment:%d", paymentID), "earn", p.No, amount)
}

// Reverse is called in the refund-success transaction, after locking its payment.
// Recompute entitlement from the net payment, rather than rounding each refund.
func Reverse(ctx context.Context, tx sqlx.Session, paymentID, refundID int64) error {
	var a struct {
		User     string `db:"user_id"`
		Awarded  int64  `db:"awarded"`
		Reversed int64  `db:"reversed"`
	}
	err := tx.QueryRowCtx(ctx, &a, `SELECT user_id,awarded,reversed FROM points_payment_award WHERE payment_id=? FOR UPDATE`, paymentID)
	if errors.Is(err, sqlx.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	var net int64
	if err = tx.QueryRowCtx(ctx, &net, `SELECT CAST(ROUND((p.amount-COALESCE((SELECT SUM(r.amount) FROM refund r WHERE r.payment_id=p.id AND r.status='success'),0))*100) AS SIGNED) FROM payment p WHERE p.id=?`, paymentID); err != nil {
		return err
	}
	target := a.Awarded - Earned(net)
	if target < 0 {
		target = 0
	}
	if target > a.Awarded {
		target = a.Awarded
	}
	delta := target - a.Reversed
	if delta <= 0 {
		return nil
	}
	if err = Change(ctx, tx, a.User, fmt.Sprintf("refund:%d", refundID), "refund", fmt.Sprintf("%d", refundID), -delta); err != nil {
		return err
	}
	_, err = tx.ExecCtx(ctx, `UPDATE points_payment_award SET reversed=? WHERE payment_id=?`, target, paymentID)
	return err
}
