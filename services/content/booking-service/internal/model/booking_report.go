package model

import (
	"context"
	"strings"
)

type BookingReportStats struct {
	BookingCount   int     `db:"booking_count"`
	CompletedCount int     `db:"completed_count"`
	MeritMoney     float64 `db:"merit_money"`
}

type BookingReportTrend struct {
	Date       string  `db:"date"`
	Bookings   int     `db:"bookings"`
	MeritMoney float64 `db:"merit_money"`
}

type BookingReportService struct {
	ServiceName string `db:"service_name"`
	Count       int    `db:"count"`
}

type BookingReportMaster struct {
	MasterCode string  `db:"master_code"`
	MasterName string  `db:"master_name"`
	Count      int     `db:"count"`
	MeritMoney float64 `db:"merit_money"`
}

func (m *defaultBookingModel) Report(ctx context.Context, templeID, start, end string) (BookingReportStats, []*BookingReportTrend, []*BookingReportService, []*BookingReportMaster, error) {
	where, args := paidBookingReportWhere(templeID, start, end)
	var stats BookingReportStats
	if err := m.conn.QueryRowCtx(ctx, &stats, `SELECT COUNT(*) booking_count,
		COALESCE(SUM(CASE WHEN status IN ('completed','reviewed') THEN 1 ELSE 0 END),0) completed_count,
		COALESCE(SUM(merit_money),0) merit_money FROM booking WHERE `+where, args...); err != nil {
		return stats, nil, nil, nil, err
	}
	var trend []*BookingReportTrend
	if err := m.conn.QueryRowsCtx(ctx, &trend, `SELECT DATE_FORMAT(create_time,'%Y-%m-%d') date,
		COUNT(*) bookings,COALESCE(SUM(merit_money),0) merit_money FROM booking WHERE `+where+
		` GROUP BY DATE_FORMAT(create_time,'%Y-%m-%d') ORDER BY date`, args...); err != nil {
		return stats, nil, nil, nil, err
	}
	var services []*BookingReportService
	if err := m.conn.QueryRowsCtx(ctx, &services, `SELECT service_name,COUNT(*) count FROM booking WHERE `+where+
		` GROUP BY service_code,service_name ORDER BY count DESC,service_name`, args...); err != nil {
		return stats, nil, nil, nil, err
	}
	var masters []*BookingReportMaster
	if err := m.conn.QueryRowsCtx(ctx, &masters, `SELECT master_code,MAX(master_name) master_name,COUNT(*) count,
		COALESCE(SUM(merit_money),0) merit_money FROM booking WHERE `+where+
		` AND master_code<>'' GROUP BY master_code ORDER BY merit_money DESC,count DESC`, args...); err != nil {
		return stats, nil, nil, nil, err
	}
	return stats, trend, services, masters, nil
}

func paidBookingReportWhere(templeID, start, end string) (string, []interface{}) {
	clauses := []string{"temple_code=?", "payment_status='success'", "status<>'cancelled'"}
	args := []interface{}{templeID}
	if start = strings.TrimSpace(start); start != "" {
		clauses = append(clauses, "create_time>=?")
		args = append(args, start+" 00:00:00")
	}
	if end = strings.TrimSpace(end); end != "" {
		clauses = append(clauses, "create_time<DATE_ADD(?,INTERVAL 1 DAY)")
		args = append(args, end+" 00:00:00")
	}
	return strings.Join(clauses, " AND "), args
}
