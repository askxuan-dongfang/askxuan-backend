package model

import (
	"context"
	"errors"

	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const noUnsettledPayment = `NOT EXISTS (
	SELECT 1 FROM askxuan_payment.payment p
	WHERE p.order_type='shop_order' AND p.order_no=o.order_no
	AND p.status IN ('pending','success','refunding','refunded')
)`

type StockRelease struct {
	ID        int64  `db:"id"`
	RequestID string `db:"request_id"`
}

// ClosePending locks the same order row as the cashier. An empty user ID limits
// this operation to the expiration worker and requires the deadline to have passed.
func (m *defaultShopOrderModel) ClosePending(ctx context.Context, id int64, userID string) error {
	return m.conn.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var o struct {
			UserID  string `db:"user_id"`
			Status  string `db:"status"`
			Expired bool   `db:"expired"`
		}
		err := tx.QueryRowCtx(ctx, &o, `SELECT user_id,status,COALESCE(payment_expire_time<=NOW(),0) expired FROM shop_order WHERE id=? FOR UPDATE`, id)
		if errors.Is(err, sqlx.ErrNotFound) {
			return common.ErrOrderNotFound
		}
		if err != nil {
			return err
		}
		if userID != "" && o.UserID != userID {
			return common.ErrForbidden
		}
		if o.Status == OrderStatusCancelled {
			return nil
		}
		if o.Status != OrderStatusPendingPayment || (userID == "" && !o.Expired) {
			return common.ErrOrderStatusConflict
		}
		result, err := tx.ExecCtx(ctx, `UPDATE shop_order o SET status='cancelled',stock_release_pending=(COALESCE(request_id,'')<>'') WHERE id=? AND `+noUnsettledPayment, id)
		if err != nil {
			return err
		}
		updated, err := result.RowsAffected()
		if err == nil && updated == 0 {
			return common.NewBizError(common.ErrOrderStatusConflict.Code, "订单已有支付记录，请先确认支付结果")
		}
		return err
	})
}

func (m *defaultShopOrderModel) FindExpired(ctx context.Context, limit int) ([]int64, error) {
	var ids []int64
	err := m.conn.QueryRowsCtx(ctx, &ids, `SELECT id FROM shop_order o WHERE status='pending_payment' AND payment_expire_time<=NOW()
		AND `+noUnsettledPayment+` ORDER BY payment_expire_time,id LIMIT ?`, limit)
	return ids, err
}

func (m *defaultShopOrderModel) FindStockReleases(ctx context.Context, limit int) ([]StockRelease, error) {
	var releases []StockRelease
	err := m.conn.QueryRowsCtx(ctx, &releases, `SELECT id,request_id FROM shop_order WHERE stock_release_pending=1 ORDER BY id LIMIT ?`, limit)
	return releases, err
}

func (m *defaultShopOrderModel) CompleteStockRelease(ctx context.Context, id int64) error {
	_, err := m.conn.ExecCtx(ctx, `UPDATE shop_order SET stock_release_pending=0,update_time=update_time WHERE id=? AND stock_release_pending=1`, id)
	return err
}
