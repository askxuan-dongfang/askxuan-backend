package model

import (
	"context"
	"fmt"
	"time"

	"github.com/askxuan/payment-service/internal/points"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// 退款状态常量
const (
	RefundStatusPending    = "pending"    // 待处理
	RefundStatusProcessing = "processing" // 处理中
	RefundStatusSuccess    = "success"    // 退款成功
	RefundStatusFailed     = "failed"     // 退款失败
)

// Refund 退款表
type Refund struct {
	Id         int64   `db:"id" json:"id"`
	RefundNo   string  `db:"refund_no" json:"refundNo"`
	PaymentId  int64   `db:"payment_id" json:"paymentId"`
	Amount     float64 `db:"amount" json:"amount"`
	Reason     string  `db:"reason" json:"reason"`
	Status     string  `db:"status" json:"status"`
	CreateTime string  `db:"create_time" json:"createTime"`
}

// RefundModel 退款模型接口
type RefundModel interface {
	Insert(ctx context.Context, data *Refund) (*Refund, error)
	FindOne(ctx context.Context, id int64) (*Refund, error)
	UpdateStatus(ctx context.Context, id int64, status string) (*Refund, error)
}

type defaultRefundModel struct {
	conn sqlx.SqlConn
}

func NewRefundModel(conn sqlx.SqlConn) RefundModel {
	return &defaultRefundModel{conn: conn}
}

// Insert 新建退款单
func (m *defaultRefundModel) Insert(ctx context.Context, data *Refund) (*Refund, error) {
	if data.RefundNo == "" {
		data.RefundNo = "RF" + time.Now().Format("20060102") + fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	}
	if data.Status == "" {
		data.Status = RefundStatusProcessing
	}
	data.CreateTime = time.Now().Format("2006-01-02 15:04:05")

	query := `INSERT INTO ` + refundTable + ` (refund_no, payment_id, amount, reason, status, create_time) VALUES (?, ?, ?, ?, ?, ?)`
	result, err := m.conn.ExecCtx(ctx, query, data.RefundNo, data.PaymentId, data.Amount, data.Reason, data.Status, data.CreateTime)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	data.Id = id
	return data, nil
}

// FindOne 按 ID 查询
func (m *defaultRefundModel) FindOne(ctx context.Context, id int64) (*Refund, error) {
	var r Refund
	query := `SELECT id, refund_no, payment_id, amount, reason, status, create_time FROM ` + refundTable + ` WHERE id = ?`
	err := m.conn.QueryRowCtx(ctx, &r, query, id)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// UpdateStatus 更新退款状态
func (m *defaultRefundModel) UpdateStatus(ctx context.Context, id int64, status string) (*Refund, error) {
	err := m.conn.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var paymentID int64
		if err := tx.QueryRowCtx(ctx, &paymentID, `SELECT payment_id FROM refund WHERE id=?`, id); err != nil {
			return err
		}
		var paymentStatus string
		if err := tx.QueryRowCtx(ctx, &paymentStatus, `SELECT status FROM payment WHERE id=? FOR UPDATE`, paymentID); err != nil {
			return err
		}
		var old string
		if err := tx.QueryRowCtx(ctx, &old, `SELECT status FROM refund WHERE id=? FOR UPDATE`, id); err != nil {
			return err
		}
		if old == status {
			return nil
		}
		if old == RefundStatusSuccess {
			return fmt.Errorf("refund already succeeded")
		}
		if _, err := tx.ExecCtx(ctx, `UPDATE refund SET status=? WHERE id=?`, status, id); err != nil {
			return err
		}
		if status == RefundStatusSuccess {
			return points.Reverse(ctx, tx, paymentID, id)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return m.FindOne(ctx, id)
}
