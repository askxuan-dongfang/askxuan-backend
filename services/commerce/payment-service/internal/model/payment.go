package model

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// 支付状态常量（参照 state-machines.md 支付状态机）
const (
	PaymentStatusPending   = "pending"   // 待支付
	PaymentStatusSuccess   = "success"   // 支付成功
	PaymentStatusFailed    = "failed"    // 支付失败
	PaymentStatusRefunding = "refunding" // 退款中
	PaymentStatusRefunded  = "refunded"  // 已退款
	PaymentStatusClosed    = "closed"    // 已关闭
)

// 支付渠道常量
const (
	PaymentChannelWechat = "wechat"
	PaymentChannelAlipay = "alipay"
)

// 订单类型常量
const (
	OrderTypeBooking   = "booking"
	OrderTypeShopOrder = "shop_order"
	OrderTypeDiyOrder  = "diy_order"
)

// paymentValidTransitions 支付合法状态流转（参照 state-machines.md 8.2）
var paymentValidTransitions = map[string]map[string]bool{
	PaymentStatusPending: {
		PaymentStatusSuccess: true,
		PaymentStatusFailed:  true,
		PaymentStatusClosed:  true,
	},
	PaymentStatusSuccess: {
		PaymentStatusRefunding: true,
	},
	PaymentStatusRefunding: {
		PaymentStatusRefunded: true,
		PaymentStatusSuccess:  true, // 退款失败恢复
	},
}

// CanPaymentTransit 校验支付状态流转是否合法
func CanPaymentTransit(from, to string) bool {
	if from == to {
		return false
	}
	allowed, ok := paymentValidTransitions[from]
	if !ok {
		return false
	}
	return allowed[to]
}

// IsPaymentTerminalStatus 是否终态
func IsPaymentTerminalStatus(s string) bool {
	return s == PaymentStatusFailed || s == PaymentStatusClosed ||
		s == PaymentStatusRefunded || s == PaymentStatusSuccess
}

// 跨库表名常量（payment 表在 askxuan_shop 库，DSN 指向 askxuan_shop，全限定名更安全）
const (
	paymentTable    = "askxuan_shop.payment"
	paymentLogTable = "askxuan_shop.payment_log"
	refundTable     = "askxuan_shop.refund"
)

// Payment 支付表
type Payment struct {
	Id         int64   `db:"id" json:"id"`
	PaymentNo  string  `db:"payment_no" json:"paymentNo"`
	UserId     string  `db:"user_id" json:"userId"`
	OrderType  string  `db:"order_type" json:"orderType"`
	OrderNo    string  `db:"order_no" json:"orderNo"`
	Amount     float64 `db:"amount" json:"amount"`
	Channel    string  `db:"channel" json:"channel"`
	Status     string  `db:"status" json:"status"`
	TradeNo    string  `db:"trade_no" json:"tradeNo"`
	CreateTime string  `db:"create_time" json:"createTime"`
}

// PaymentModel 支付模型接口
type PaymentModel interface {
	Insert(ctx context.Context, data *Payment) (*Payment, error)
	FindOne(ctx context.Context, id int64) (*Payment, error)
	FindByPaymentNo(ctx context.Context, paymentNo string) (*Payment, error)
	UpdateStatus(ctx context.Context, id int64, status, tradeNo string) (*Payment, error)
}

type defaultPaymentModel struct {
	conn sqlx.SqlConn
}

func NewPaymentModel(conn sqlx.SqlConn) PaymentModel {
	return &defaultPaymentModel{conn: conn}
}

// Insert 新建支付单，返回带 ID/单号/初始状态的对象
func (m *defaultPaymentModel) Insert(ctx context.Context, data *Payment) (*Payment, error) {
	if data.PaymentNo == "" {
		data.PaymentNo = "PAY" + time.Now().Format("20060102") + fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	}
	if data.Status == "" {
		data.Status = PaymentStatusPending
	}
	data.CreateTime = time.Now().Format("2006-01-02 15:04:05")

	const query = `INSERT INTO ` + paymentTable + ` (payment_no, user_id, order_type, order_no, amount, channel, status, trade_no, create_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	result, err := m.conn.ExecCtx(ctx, query, data.PaymentNo, data.UserId, data.OrderType, data.OrderNo, data.Amount, data.Channel, data.Status, data.TradeNo, data.CreateTime)
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
func (m *defaultPaymentModel) FindOne(ctx context.Context, id int64) (*Payment, error) {
	var p Payment
	query := `SELECT id, payment_no, user_id, order_type, order_no, amount, channel, status, trade_no, create_time FROM ` + paymentTable + ` WHERE id = ?`
	err := m.conn.QueryRowCtx(ctx, &p, query, id)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// FindByPaymentNo 按支付单号查询
func (m *defaultPaymentModel) FindByPaymentNo(ctx context.Context, paymentNo string) (*Payment, error) {
	var p Payment
	query := `SELECT id, payment_no, user_id, order_type, order_no, amount, channel, status, trade_no, create_time FROM ` + paymentTable + ` WHERE payment_no = ?`
	err := m.conn.QueryRowCtx(ctx, &p, query, paymentNo)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// UpdateStatus 更新支付状态与第三方交易号（调用方需先校验 CanPaymentTransit）
func (m *defaultPaymentModel) UpdateStatus(ctx context.Context, id int64, status, tradeNo string) (*Payment, error) {
	if tradeNo != "" {
		query := `UPDATE ` + paymentTable + ` SET status = ?, trade_no = ? WHERE id = ?`
		_, err := m.conn.ExecCtx(ctx, query, status, tradeNo, id)
		if err != nil {
			return nil, err
		}
	} else {
		query := `UPDATE ` + paymentTable + ` SET status = ? WHERE id = ?`
		_, err := m.conn.ExecCtx(ctx, query, status, id)
		if err != nil {
			return nil, err
		}
	}
	return m.FindOne(ctx, id)
}

// PaymentLog 支付日志表
type PaymentLog struct {
	Id         int64  `db:"id" json:"id"`
	PaymentId  int64  `db:"payment_id" json:"paymentId"`
	Action     string `db:"action" json:"action"`
	Request    string `db:"request" json:"request"`
	Response   string `db:"response" json:"response"`
	CreateTime string `db:"create_time" json:"createTime"`
}

// PaymentLogModel 支付日志模型接口
type PaymentLogModel interface {
	Insert(ctx context.Context, data *PaymentLog) error
	ListByPaymentId(ctx context.Context, paymentId int64) ([]*PaymentLog, error)
}

type defaultPaymentLogModel struct {
	conn sqlx.SqlConn
}

func NewPaymentLogModel(conn sqlx.SqlConn) PaymentLogModel {
	return &defaultPaymentLogModel{conn: conn}
}

func (m *defaultPaymentLogModel) Insert(ctx context.Context, data *PaymentLog) error {
	query := `INSERT INTO ` + paymentLogTable + ` (payment_id, action, request, response, create_time) VALUES (?, ?, ?, ?, NOW())`
	_, err := m.conn.ExecCtx(ctx, query, data.PaymentId, data.Action, data.Request, data.Response)
	return err
}

func (m *defaultPaymentLogModel) ListByPaymentId(ctx context.Context, paymentId int64) ([]*PaymentLog, error) {
	query := `SELECT id, payment_id, action, request, response, create_time FROM ` + paymentLogTable + ` WHERE payment_id = ? ORDER BY create_time ASC`
	var logs []*PaymentLog
	err := m.conn.QueryRowsCtx(ctx, &logs, query, paymentId)
	if err != nil {
		return nil, err
	}
	return logs, nil
}
