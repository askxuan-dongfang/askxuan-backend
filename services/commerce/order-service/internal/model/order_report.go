package model

import (
	"context"
	"fmt"
	"strings"
)

// OrderReportRow 日报聚合行
type OrderReportRow struct {
	Date   string  `db:"date"`
	Sales  float64 `db:"sales"`
	Orders int     `db:"orders"`
}

// OrderTopProduct 热销商品
type OrderTopProduct struct {
	ProductId   int64   `db:"product_id"`
	ProductName string  `db:"product_name"`
	Sales       float64 `db:"sales"`
	OrderCount  int     `db:"order_count"`
}

// ReportStats 报表统计（今日/待发货）
type ReportStats struct {
	TodayOrders int     `db:"today_orders"`
	TodaySales  float64 `db:"today_sales"`
	PendingShip int     `db:"pending_ship"`
	TotalOrders int     `db:"total_orders"`
	TotalSales  float64 `db:"total_sales"`
}

// GetReportStats 今日/待发货/累计统计
func (m *defaultShopOrderModel) GetReportStats(ctx context.Context, start, end string) (*ReportStats, error) {
	var s ReportStats
	where, args := paidOrderReportWhere("create_time", "status", start, end)
	query := fmt.Sprintf(`SELECT
		COALESCE(SUM(CASE WHEN DATE(create_time)=CURDATE() THEN 1 ELSE 0 END),0) AS today_orders,
		COALESCE(SUM(CASE WHEN DATE(create_time)=CURDATE() THEN pay_amount ELSE 0 END),0) AS today_sales,
		COALESCE(SUM(CASE WHEN status='paid' THEN 1 ELSE 0 END),0) AS pending_ship,
		COUNT(*) AS total_orders,
		COALESCE(SUM(pay_amount),0) AS total_sales
		FROM %s WHERE %s`, shopOrderTable, where)
	if err := m.conn.QueryRowCtx(ctx, &s, query, args...); err != nil {
		return nil, err
	}
	return &s, nil
}

// GetReportTrend 近 7 天销售趋势
func (m *defaultShopOrderModel) GetReportTrend(ctx context.Context, start, end string) ([]*OrderReportRow, error) {
	rows := make([]*OrderReportRow, 0)
	where, args := paidOrderReportWhere("create_time", "status", start, end)
	query := fmt.Sprintf(`SELECT DATE_FORMAT(create_time,'%%Y-%%m-%%d') AS date,
		COALESCE(SUM(pay_amount),0) AS sales, COUNT(*) AS orders
		FROM %s
		WHERE %s
		GROUP BY DATE_FORMAT(create_time,'%%Y-%%m-%%d')
		ORDER BY date ASC`, shopOrderTable, where)
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

// GetReportTopProducts 热销商品 Top5
func (m *defaultShopOrderModel) GetReportTopProducts(ctx context.Context, start, end string, limit int) ([]*OrderTopProduct, error) {
	rows := make([]*OrderTopProduct, 0)
	where, args := paidOrderReportWhere("o.create_time", "o.status", start, end)
	query := fmt.Sprintf(`SELECT i.product_id, MAX(i.product_name) AS product_name,
		COALESCE(SUM(i.price*i.quantity),0) AS sales,
		COUNT(DISTINCT i.order_id) AS order_count
		FROM shop_order_item i
		JOIN %s o ON o.id = i.order_id
		WHERE %s
		GROUP BY i.product_id
		ORDER BY sales DESC
		LIMIT ?`, shopOrderTable, where)
	args = append(args, limit)
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	return rows, nil
}

func (m *defaultShopOrderModel) GetReportReturnRate(ctx context.Context, start, end string) (float64, error) {
	where, args := paidOrderReportWhere("o.create_time", "o.status", start, end)
	var row struct {
		Orders  int `db:"orders"`
		Returns int `db:"returns"`
	}
	query := fmt.Sprintf(`SELECT COUNT(DISTINCT o.id) orders,
		COUNT(DISTINCT CASE WHEN r.status<>'rejected' THEN r.order_id END) returns
		FROM %s o LEFT JOIN return_order r ON r.order_id=o.id WHERE %s`, shopOrderTable, where)
	if err := m.conn.QueryRowCtx(ctx, &row, query, args...); err != nil {
		return 0, err
	}
	if row.Orders == 0 {
		return 0, nil
	}
	return float64(row.Returns) / float64(row.Orders), nil
}

func paidOrderReportWhere(timeColumn, statusColumn, start, end string) (string, []interface{}) {
	clauses := []string{statusColumn + " IN ('paid','shipped','completed','in_return')"}
	args := []interface{}{}
	if start = strings.TrimSpace(start); start != "" {
		clauses = append(clauses, timeColumn+">=?")
		args = append(args, start+" 00:00:00")
	}
	if end = strings.TrimSpace(end); end != "" {
		clauses = append(clauses, timeColumn+"<DATE_ADD(?,INTERVAL 1 DAY)")
		args = append(args, end+" 00:00:00")
	}
	return strings.Join(clauses, " AND "), args
}
