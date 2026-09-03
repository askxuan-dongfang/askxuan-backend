package model

import (
	"context"
	"strings"
)

type FinancialReport struct {
	GrossIncome    float64 `db:"gross_income"`
	RefundAmount   float64 `db:"refund_amount"`
	SettlementNet  float64 `db:"settlement_net"`
	WithdrawalPaid float64 `db:"withdrawal_paid"`
	OrderCount     int     `db:"order_count"`
}

type ShopTrend struct {
	Date   string  `db:"date"`
	Sales  float64 `db:"sales"`
	Orders int     `db:"orders"`
}

func QueryFinancialReport(ctx context.Context, start, end string) (FinancialReport, error) {
	where, args := reportTimeWhere("create_time", start, end)
	var result FinancialReport
	if err := db.QueryRowCtx(ctx, &result.GrossIncome, `SELECT COALESCE(SUM(total_amount),0) FROM finance_transaction WHERE event_type='payment_receipt' AND status='posted' AND `+where, args...); err != nil {
		return result, err
	}
	if err := db.QueryRowCtx(ctx, &result.RefundAmount, `SELECT COALESCE(SUM(total_amount),0) FROM finance_transaction WHERE event_type='refund' AND status='posted' AND `+where, args...); err != nil {
		return result, err
	}
	if err := db.QueryRowCtx(ctx, &result.OrderCount, `SELECT COUNT(DISTINCT source_type,source_no) FROM finance_transaction WHERE event_type='payment_receipt' AND status='posted' AND `+where, args...); err != nil {
		return result, err
	}
	settleWhere, settleArgs := reportTimeWhere("create_time", start, end)
	if err := db.QueryRowCtx(ctx, &result.SettlementNet, `SELECT COALESCE(SUM(settle_amount),0) FROM settlement WHERE `+settleWhere, settleArgs...); err != nil {
		return result, err
	}
	withdrawWhere, withdrawArgs := reportTimeWhere("process_time", start, end)
	if err := db.QueryRowCtx(ctx, &result.WithdrawalPaid, `SELECT COALESCE(SUM(amount),0) FROM withdrawal WHERE status='success' AND `+withdrawWhere, withdrawArgs...); err != nil {
		return result, err
	}
	return result, nil
}

func QuerySettlementNetByType(ctx context.Context, start, end string) (map[string]float64, error) {
	where, args := reportTimeWhere("create_time", start, end)
	var rows []struct {
		Type   string  `db:"settle_type"`
		Amount float64 `db:"amount"`
	}
	if err := db.QueryRowsCtx(ctx, &rows, `SELECT settle_type,COALESCE(SUM(settle_amount),0) amount FROM settlement WHERE `+where+` GROUP BY settle_type`, args...); err != nil {
		return nil, err
	}
	out := map[string]float64{}
	for _, row := range rows {
		out[row.Type] = row.Amount
	}
	return out, nil
}

func QueryCommission(ctx context.Context, start, end string) (float64, error) {
	where, args := reportTimeWhere("create_time", start, end)
	var amount float64
	err := db.QueryRowCtx(ctx, &amount, `SELECT COALESCE(SUM(commission_amount),0) FROM settlement WHERE `+where, args...)
	return amount, err
}

func QueryShopReport(ctx context.Context, start, end string) (gross, refunds float64, orders, refundOrders int, trend []*ShopTrend, err error) {
	where, args := reportTimeWhere("create_time", start, end)
	if err = db.QueryRowCtx(ctx, &gross, `SELECT COALESCE(SUM(total_amount),0) FROM finance_transaction WHERE source_type='shop_order' AND event_type='payment_receipt' AND status='posted' AND `+where, args...); err != nil {
		return
	}
	if err = db.QueryRowCtx(ctx, &orders, `SELECT COUNT(*) FROM finance_transaction WHERE source_type='shop_order' AND event_type='payment_receipt' AND status='posted' AND `+where, args...); err != nil {
		return
	}
	if err = db.QueryRowCtx(ctx, &refunds, `SELECT COALESCE(SUM(total_amount),0) FROM finance_transaction WHERE source_type='shop_order' AND event_type='refund' AND status='posted' AND `+where, args...); err != nil {
		return
	}
	if err = db.QueryRowCtx(ctx, &refundOrders, `SELECT COUNT(*) FROM finance_transaction WHERE source_type='shop_order' AND event_type='refund' AND status='posted' AND `+where, args...); err != nil {
		return
	}
	err = db.QueryRowsCtx(ctx, &trend, `SELECT DATE_FORMAT(create_time,'%Y-%m-%d') date,COALESCE(SUM(total_amount),0) sales,COUNT(*) orders FROM finance_transaction WHERE source_type='shop_order' AND event_type='payment_receipt' AND status='posted' AND `+where+` GROUP BY DATE_FORMAT(create_time,'%Y-%m-%d') ORDER BY date`, args...)
	return
}

func reportTimeWhere(column, start, end string) (string, []interface{}) {
	clauses := []string{"1=1"}
	args := []interface{}{}
	if start = strings.TrimSpace(start); start != "" {
		clauses = append(clauses, column+">=?")
		args = append(args, start+" 00:00:00")
	}
	if end = strings.TrimSpace(end); end != "" {
		clauses = append(clauses, column+"<DATE_ADD(?,INTERVAL 1 DAY)")
		args = append(args, end+" 00:00:00")
	}
	return strings.Join(clauses, " AND "), args
}
