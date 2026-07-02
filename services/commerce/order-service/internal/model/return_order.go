package model

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// 退换货类型常量
const (
	ReturnTypeReturn   = "return"   // 退货退款
	ReturnTypeExchange = "exchange" // 换货
)

// 退换货状态常量（参照 state-machines.md 退换货状态机）
const (
	ReturnStatusPendingReview  = "pending_review"  // 待审核
	ReturnStatusApproved       = "approved"        // 已同意
	ReturnStatusReturnShipping = "return_shipping" // 退货运输中
	ReturnStatusReturnReceived = "return_received" // 已收货
	ReturnStatusRefunding      = "refunding"       // 退款中
	ReturnStatusCompleted      = "completed"       // 已完成
	ReturnStatusRejected       = "rejected"        // 已拒绝
)

// returnValidTransitions 退换货合法状态流转
var returnValidTransitions = map[string]map[string]bool{
	ReturnStatusPendingReview: {
		ReturnStatusApproved: true,
		ReturnStatusRejected: true,
	},
	ReturnStatusApproved: {
		ReturnStatusReturnShipping: true,
	},
	ReturnStatusReturnShipping: {
		ReturnStatusReturnReceived: true,
	},
	ReturnStatusReturnReceived: {
		ReturnStatusRefunding: true,
	},
	ReturnStatusRefunding: {
		ReturnStatusCompleted: true,
	},
}

// CanReturnTransit 校验退换货状态流转是否合法
func CanReturnTransit(from, to string) bool {
	if from == to {
		return false
	}
	allowed, ok := returnValidTransitions[from]
	if !ok {
		return false
	}
	return allowed[to]
}

const returnOrderTable = "askxuan_shop.return_order"

// ReturnOrder 退换货表
type ReturnOrder struct {
	Id           int64   `db:"id" json:"id"`
	ReturnNo     string  `db:"return_no" json:"returnNo"`
	OrderId      int64   `db:"order_id" json:"orderId"`
	Type         string  `db:"type" json:"type"`
	Reason       string  `db:"reason" json:"reason"`
	Status       string  `db:"status" json:"status"`
	RefundAmount float64 `db:"refund_amount" json:"refundAmount"`
	CreateTime   string  `db:"create_time" json:"createTime"`
	UpdateTime   string  `db:"update_time" json:"updateTime"`
}

// ReturnOrderModel 退换货模型接口
type ReturnOrderModel interface {
	Insert(ctx context.Context, data *ReturnOrder) (*ReturnOrder, error)
	FindOne(ctx context.Context, id int64) (*ReturnOrder, error)
	FindList(ctx context.Context, status string, page, size int) ([]*ReturnOrder, int64, error)
	UpdateStatus(ctx context.Context, id int64, status string) (*ReturnOrder, error)
}

type defaultReturnOrderModel struct {
	conn sqlx.SqlConn
}

func NewReturnOrderModel(conn sqlx.SqlConn) ReturnOrderModel {
	return &defaultReturnOrderModel{conn: conn}
}

func (m *defaultReturnOrderModel) Insert(ctx context.Context, data *ReturnOrder) (*ReturnOrder, error) {
	if data.ReturnNo == "" {
		data.ReturnNo = "RO" + time.Now().Format("20060102") + fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	}
	if data.Status == "" {
		data.Status = ReturnStatusPendingReview
	}
	if data.Type == "" {
		data.Type = ReturnTypeReturn
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	data.CreateTime = now
	data.UpdateTime = now

	query := fmt.Sprintf(`INSERT INTO %s (return_no, order_id, type, reason, status, refund_amount, create_time, update_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, returnOrderTable)
	result, err := m.conn.ExecCtx(ctx, query, data.ReturnNo, data.OrderId, data.Type, data.Reason, data.Status, data.RefundAmount, data.CreateTime, data.UpdateTime)
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

func (m *defaultReturnOrderModel) FindOne(ctx context.Context, id int64) (*ReturnOrder, error) {
	var r ReturnOrder
	query := fmt.Sprintf(`SELECT id, return_no, order_id, type, reason, status, refund_amount, create_time, update_time FROM %s WHERE id = ?`, returnOrderTable)
	err := m.conn.QueryRowCtx(ctx, &r, query, id)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (m *defaultReturnOrderModel) FindList(ctx context.Context, status string, page, size int) ([]*ReturnOrder, int64, error) {
	where := "1=1"
	var args []interface{}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(1) FROM %s WHERE %s`, returnOrderTable, where)
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*ReturnOrder{}, 0, nil
	}

	offset := (page - 1) * size
	listQuery := fmt.Sprintf(`SELECT id, return_no, order_id, type, reason, status, refund_amount, create_time, update_time FROM %s WHERE %s ORDER BY create_time DESC LIMIT ?, ?`, returnOrderTable, where)
	listArgs := append(args, offset, size)
	var list []*ReturnOrder
	if err := m.conn.QueryRowsCtx(ctx, &list, listQuery, listArgs...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *defaultReturnOrderModel) UpdateStatus(ctx context.Context, id int64, status string) (*ReturnOrder, error) {
	query := fmt.Sprintf(`UPDATE %s SET status=?, update_time=? WHERE id=?`, returnOrderTable)
	_, err := m.conn.ExecCtx(ctx, query, status, time.Now().Format("2006-01-02 15:04:05"), id)
	if err != nil {
		return nil, err
	}
	return m.FindOne(ctx, id)
}
