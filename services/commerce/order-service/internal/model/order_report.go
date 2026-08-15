package model

import (
	"context"
	"fmt"
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
func (m *defaultShopOrderModel) GetReportStats(ctx context.Context) (*ReportStats, error) {
	var s ReportStats
	query := fmt.Sprintf(`SELECT
		COALESCE(SUM(CASE WHEN DATE(create_time)=CURDATE() AND status<>'cancelled' THEN 1 ELSE 0 END),0) AS today_orders,
		COALESCE(SUM(CASE WHEN DATE(create_time)=CURDATE() AND status<>'cancelled' THEN pay_amount ELSE 0 END),0) AS today_sales,
		COALESCE(SUM(CASE WHEN status='paid' THEN 1 ELSE 0 END),0) AS pending_ship,
		COALESCE(SUM(CASE WHEN status<>'cancelled' THEN 1 ELSE 0 END),0) AS total_orders,
		COALESCE(SUM(CASE WHEN status<>'cancelled' THEN pay_amount ELSE 0 END),0) AS total_sales
		FROM %s`, shopOrderTable)
	if err := m.conn.QueryRowCtx(ctx, &s, query); err != nil {
		return nil, err
	}
	return &s, nil
}

// GetReportTrend 近 7 天销售趋势
func (m *defaultShopOrderModel) GetReportTrend(ctx context.Context, days int) ([]*OrderReportRow, error) {
	rows := make([]*OrderReportRow, 0)
	query := fmt.Sprintf(`SELECT DATE_FORMAT(create_time,'%%Y-%%m-%%d') AS date,
		COALESCE(SUM(pay_amount),0) AS sales, COUNT(*) AS orders
		FROM %s
		WHERE status<>'cancelled' AND create_time >= DATE_SUB(CURDATE(), INTERVAL ? DAY)
		GROUP BY DATE_FORMAT(create_time,'%%Y-%%m-%%d')
		ORDER BY date ASC`, shopOrderTable)
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, days-1); err != nil {
		return nil, err
	}
	return rows, nil
}

// GetReportTopProducts 热销商品 Top5
func (m *defaultShopOrderModel) GetReportTopProducts(ctx context.Context, limit int) ([]*OrderTopProduct, error) {
	rows := make([]*OrderTopProduct, 0)
	query := fmt.Sprintf(`SELECT i.product_id, MAX(i.product_name) AS product_name,
		COALESCE(SUM(i.price*i.quantity),0) AS sales,
		COUNT(DISTINCT i.order_id) AS order_count
		FROM shop_order_item i
		JOIN %s o ON o.id = i.order_id AND o.status<>'cancelled'
		GROUP BY i.product_id
		ORDER BY sales DESC
		LIMIT ?`, shopOrderTable)
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, limit); err != nil {
		return nil, err
	}
	return rows, nil
}
