package model

import (
	"context"
	"errors"
	"math"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var (
	ErrDiyOrderOwnerMismatch = errors.New("diy order owner mismatch")
	ErrDiyOrderNotPayable    = errors.New("diy order is not payable")
	ErrDiyOrderAmountChanged = errors.New("diy order amount changed")
)

type DiyPaymentOrderModel interface {
	ValidatePayment(ctx context.Context, orderNo, userId string, amount float64) error
}

type diyPaymentOrderModel struct {
	conn sqlx.SqlConn
}

type diyPaymentOrder struct {
	UserId        string  `db:"user_id"`
	TotalFee      float64 `db:"total_fee"`
	Status        string  `db:"status"`
	PaymentStatus string  `db:"payment_status"`
}

func NewDiyPaymentOrderModel(conn sqlx.SqlConn) DiyPaymentOrderModel {
	return &diyPaymentOrderModel{conn: conn}
}

func (m *diyPaymentOrderModel) ValidatePayment(ctx context.Context, orderNo, userId string, amount float64) error {
	var order diyPaymentOrder
	const query = `SELECT user_id,total_fee,status,payment_status FROM askxuan_diy.diy_order WHERE order_no = ? LIMIT 1`
	if err := m.conn.QueryRowCtx(ctx, &order, query, orderNo); err != nil {
		return err
	}
	return validateDiyPaymentOrder(order, userId, amount)
}

func validateDiyPaymentOrder(order diyPaymentOrder, userId string, amount float64) error {
	if order.UserId != userId {
		return ErrDiyOrderOwnerMismatch
	}
	if order.Status != "pending_review" {
		return ErrDiyOrderNotPayable
	}
	if order.PaymentStatus == "success" {
		return ErrDiyOrderNotPayable
	}
	if math.Abs(order.TotalFee-amount) >= 0.005 {
		return ErrDiyOrderAmountChanged
	}
	return nil
}
