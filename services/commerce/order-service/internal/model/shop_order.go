package model

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// 商城订单状态常量（参照 state-machines.md 商城订单状态机）
const (
	OrderStatusPendingPayment = "pending_payment" // 待付款
	OrderStatusPaid           = "paid"            // 已支付
	OrderStatusShipped        = "shipped"         // 已发货
	OrderStatusCompleted      = "completed"       // 已完成
	OrderStatusCancelled      = "cancelled"       // 已取消
	OrderStatusInReturn       = "in_return"       // 售后中
)

// orderValidTransitions 商城订单合法状态流转
var orderValidTransitions = map[string]map[string]bool{
	OrderStatusPendingPayment: {
		OrderStatusPaid:      true,
		OrderStatusCancelled: true,
	},
	OrderStatusPaid: {
		OrderStatusShipped:   true,
		OrderStatusCancelled: true,
	},
	OrderStatusShipped: {
		OrderStatusCompleted: true,
		OrderStatusInReturn:  true,
	},
	OrderStatusInReturn: {
		OrderStatusCompleted: true,
		OrderStatusShipped:   true,
	},
}

// CanOrderTransit 校验订单状态流转是否合法
func CanOrderTransit(from, to string) bool {
	if from == to {
		return false
	}
	allowed, ok := orderValidTransitions[from]
	if !ok {
		return false
	}
	return allowed[to]
}

// IsOrderTerminalStatus 是否终态
func IsOrderTerminalStatus(s string) bool {
	return s == OrderStatusCompleted || s == OrderStatusCancelled
}

const shopOrderTable = "shop_order"

// ShopOrder 商城订单表
type ShopOrder struct {
	Id          int64   `db:"id" json:"id"`
	RequestId   string  `db:"request_id" json:"requestId"`
	OrderNo     string  `db:"order_no" json:"orderNo"`
	UserId      string  `db:"user_id" json:"userId"`
	TotalAmount float64 `db:"total_amount" json:"totalAmount"`
	PayAmount   float64 `db:"pay_amount" json:"payAmount"`
	Status      string  `db:"status" json:"status"`
	AddressId   int64   `db:"address_id" json:"addressId"`
	Note        string  `db:"note" json:"note"`
	CreateTime  string  `db:"create_time" json:"createTime"`
	UpdateTime  string  `db:"update_time" json:"updateTime"`
}

// ShopOrderModel 订单模型接口
type ShopOrderModel interface {
	Insert(ctx context.Context, data *ShopOrder) (*ShopOrder, error)
	InsertWithSession(ctx context.Context, session sqlx.Session, data *ShopOrder) (*ShopOrder, error)
	FindOne(ctx context.Context, id int64) (*ShopOrder, error)
	FindByOrderNo(ctx context.Context, orderNo string) (*ShopOrder, error)
	FindByRequestId(ctx context.Context, requestId string) (*ShopOrder, error)
	FindListByUser(ctx context.Context, userId, status string, page, size int) ([]*ShopOrder, int64, error)
	FindListAdmin(ctx context.Context, status string, page, size int) ([]*ShopOrder, int64, error)
	UpdateStatus(ctx context.Context, id int64, status string) (*ShopOrder, error)
	GetReportStats(ctx context.Context) (*ReportStats, error)
	GetReportTrend(ctx context.Context, days int) ([]*OrderReportRow, error)
	GetReportTopProducts(ctx context.Context, limit int) ([]*OrderTopProduct, error)
}

type defaultShopOrderModel struct {
	conn sqlx.SqlConn
}

func NewShopOrderModel(conn sqlx.SqlConn) ShopOrderModel {
	return &defaultShopOrderModel{conn: conn}
}

func (m *defaultShopOrderModel) Insert(ctx context.Context, data *ShopOrder) (*ShopOrder, error) {
	return m.insert(ctx, m.conn, data)
}

func (m *defaultShopOrderModel) InsertWithSession(ctx context.Context, session sqlx.Session, data *ShopOrder) (*ShopOrder, error) {
	return m.insert(ctx, session, data)
}

type sqlExecutor interface {
	ExecCtx(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

func (m *defaultShopOrderModel) insert(ctx context.Context, executor sqlExecutor, data *ShopOrder) (*ShopOrder, error) {
	if data.OrderNo == "" {
		data.OrderNo = "O" + time.Now().Format("20060102") + fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	}
	if data.Status == "" {
		data.Status = OrderStatusPendingPayment
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	data.CreateTime = now
	data.UpdateTime = now

	query := fmt.Sprintf(`INSERT INTO %s (order_no, request_id, user_id, total_amount, pay_amount, status, address_id, note, create_time, update_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, shopOrderTable)
	result, err := executor.ExecCtx(ctx, query, data.OrderNo, data.RequestId, data.UserId, data.TotalAmount, data.PayAmount, data.Status, data.AddressId, data.Note, data.CreateTime, data.UpdateTime)
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

func (m *defaultShopOrderModel) FindByRequestId(ctx context.Context, requestId string) (*ShopOrder, error) {
	var o ShopOrder
	query := fmt.Sprintf(`SELECT id, order_no, COALESCE(request_id,'') AS request_id, user_id, total_amount, pay_amount, status, address_id, note, create_time, update_time FROM %s WHERE request_id = ?`, shopOrderTable)
	if err := m.conn.QueryRowCtx(ctx, &o, query, requestId); err != nil {
		return nil, err
	}
	return &o, nil
}

func (m *defaultShopOrderModel) FindOne(ctx context.Context, id int64) (*ShopOrder, error) {
	var o ShopOrder
	query := fmt.Sprintf(`SELECT id, order_no, COALESCE(request_id,'') AS request_id, user_id, total_amount, pay_amount, status, address_id, note, create_time, update_time FROM %s WHERE id = ?`, shopOrderTable)
	err := m.conn.QueryRowCtx(ctx, &o, query, id)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (m *defaultShopOrderModel) FindByOrderNo(ctx context.Context, orderNo string) (*ShopOrder, error) {
	var o ShopOrder
	query := fmt.Sprintf(`SELECT id, order_no, COALESCE(request_id,'') AS request_id, user_id, total_amount, pay_amount, status, address_id, note, create_time, update_time FROM %s WHERE order_no = ?`, shopOrderTable)
	err := m.conn.QueryRowCtx(ctx, &o, query, orderNo)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (m *defaultShopOrderModel) FindListByUser(ctx context.Context, userId, status string, page, size int) ([]*ShopOrder, int64, error) {
	where, args := buildOrderWhere(userId, status)

	countQuery := fmt.Sprintf(`SELECT COUNT(1) FROM %s WHERE %s`, shopOrderTable, where)
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*ShopOrder{}, 0, nil
	}

	offset := (page - 1) * size
	listQuery := fmt.Sprintf(`SELECT id, order_no, COALESCE(request_id,'') AS request_id, user_id, total_amount, pay_amount, status, address_id, note, create_time, update_time FROM %s WHERE %s ORDER BY create_time DESC LIMIT ?, ?`, shopOrderTable, where)
	listArgs := append(args, offset, size)
	var list []*ShopOrder
	if err := m.conn.QueryRowsCtx(ctx, &list, listQuery, listArgs...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *defaultShopOrderModel) FindListAdmin(ctx context.Context, status string, page, size int) ([]*ShopOrder, int64, error) {
	where, args := buildOrderWhere("", status)

	countQuery := fmt.Sprintf(`SELECT COUNT(1) FROM %s WHERE %s`, shopOrderTable, where)
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*ShopOrder{}, 0, nil
	}

	offset := (page - 1) * size
	listQuery := fmt.Sprintf(`SELECT id, order_no, COALESCE(request_id,'') AS request_id, user_id, total_amount, pay_amount, status, address_id, note, create_time, update_time FROM %s WHERE %s ORDER BY create_time DESC LIMIT ?, ?`, shopOrderTable, where)
	listArgs := append(args, offset, size)
	var list []*ShopOrder
	if err := m.conn.QueryRowsCtx(ctx, &list, listQuery, listArgs...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *defaultShopOrderModel) UpdateStatus(ctx context.Context, id int64, status string) (*ShopOrder, error) {
	query := fmt.Sprintf(`UPDATE %s SET status=?, update_time=? WHERE id=?`, shopOrderTable)
	_, err := m.conn.ExecCtx(ctx, query, status, time.Now().Format("2006-01-02 15:04:05"), id)
	if err != nil {
		return nil, err
	}
	return m.FindOne(ctx, id)
}

func buildOrderWhere(userId, status string) (string, []interface{}) {
	where := "1=1"
	var args []interface{}
	if userId != "" {
		where += " AND user_id = ?"
		args = append(args, userId)
	}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	return where, args
}
