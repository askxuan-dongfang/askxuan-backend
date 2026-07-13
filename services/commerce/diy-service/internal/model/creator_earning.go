package model

import (
	"context"
	"errors"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const CreatorEarningStatusPending = "pending"

type CreatorEarningModel interface {
	RecordPaymentSuccess(ctx context.Context, orderNo, paymentNo string) error
}

type creatorEarningModel struct {
	conn sqlx.SqlConn
}

func NewCreatorEarningModel(conn sqlx.SqlConn) CreatorEarningModel {
	return &creatorEarningModel{conn: conn}
}

func (m *creatorEarningModel) RecordPaymentSuccess(ctx context.Context, orderNo, paymentNo string) error {
	return m.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		var order DiyOrder
		query := `SELECT ` + diyOrderRows + ` FROM diy_order WHERE order_no=? FOR UPDATE`
		if err := session.QueryRowCtx(ctx, &order, query, orderNo); err != nil {
			if errors.Is(err, sqlx.ErrNotFound) {
				return nil
			}
			return err
		}
		if _, err := session.ExecCtx(ctx, `UPDATE diy_order SET payment_status='success',update_time=CURRENT_TIMESTAMP WHERE id=? AND payment_status<>'success'`, order.Id); err != nil {
			return err
		}
		if order.Source != "design_square" || order.DesignId == 0 || order.CreatorId == "" {
			return nil
		}
		earningAmount := roundMoney(order.MaterialFee * order.CreatorShareRate)
		_, err := session.ExecCtx(ctx, `INSERT IGNORE INTO diy_creator_earning(earning_no,order_id,order_no,design_id,creator_id,payment_no,base_amount,share_rate,earning_amount,status) VALUES(?,?,?,?,?,?,?,?,?,?)`,
			earningNo(order.Id), order.Id, order.OrderNo, order.DesignId, order.CreatorId, paymentNo, order.MaterialFee, order.CreatorShareRate, earningAmount, CreatorEarningStatusPending)
		return err
	})
}
