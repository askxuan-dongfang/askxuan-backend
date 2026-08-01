package model

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	EarningsSettlePending  = "pending"
	EarningsSettleSettled  = "settled"
	EarningsSettleWithdrew = "withdrew"
)

const (
	EarningsServiceBooking  = "booking"
	EarningsServiceBlessing = "diy_blessing"
	EarningsServiceConsult  = "consult"
)

type EarningsDetail struct {
	Id           int64   `db:"id" json:"id"`
	MasterCode   string  `db:"master_code" json:"masterCode"`
	Date         string  `db:"earning_date" json:"date"`
	ServiceType  string  `db:"service_type" json:"serviceType"`
	UserName     string  `db:"user_name" json:"userName"`
	Amount       float64 `db:"amount" json:"amount"`
	SettleStatus string  `db:"settle_status" json:"settleStatus"`
	CreateTime   string  `db:"create_time" json:"createTime"`
}

type EarningsSummary struct {
	MonthIncome  float64
	TotalIncome  float64
	Withdrawable float64
	Trend        []EarningsTrendRow
}

type EarningsTrendRow struct {
	Month  string  `db:"month" json:"month"`
	Amount float64 `db:"amount" json:"amount"`
}

var earningsDB sqlx.SqlConn

func ConfigureEarnings(conn sqlx.SqlConn) {
	earningsDB = conn
}

func RecordBookingEarning(ctx context.Context, sourceID, masterCode, earningDate, serviceName, userName string, amount float64) error {
	_, err := earningsDB.ExecCtx(ctx, `INSERT INTO master_earning
		(source_type,source_id,master_code,earning_date,service_type,service_name,user_name,amount,settle_status)
		VALUES (?,?,?,?,?,?,?,?,?)
		ON DUPLICATE KEY UPDATE
		master_code=VALUES(master_code),earning_date=VALUES(earning_date),service_name=VALUES(service_name),
		user_name=VALUES(user_name),amount=IF(settle_status='pending',VALUES(amount),amount)`,
		EarningsServiceBooking, sourceID, masterCode, earningDate, EarningsServiceBooking,
		serviceName, userName, amount, EarningsSettlePending)
	return err
}

func ListEarnings(ctx context.Context, masterCode, serviceType string, page, size int) ([]EarningsDetail, int64, error) {
	where := "master_code=?"
	args := []any{masterCode}
	if serviceType != "" {
		where += " AND service_type=?"
		args = append(args, serviceType)
	}
	var total int64
	if err := earningsDB.QueryRowCtx(ctx, &total, "SELECT COUNT(*) FROM master_earning WHERE "+where, args...); err != nil {
		return nil, 0, err
	}
	queryArgs := append(append([]any{}, args...), size, (page-1)*size)
	var list []EarningsDetail
	err := earningsDB.QueryRowsCtx(ctx, &list, `SELECT id,master_code,DATE_FORMAT(earning_date,'%Y-%m-%d') earning_date,
		service_type,user_name,amount,settle_status,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') create_time
		FROM master_earning WHERE `+where+` ORDER BY earning_date DESC,id DESC LIMIT ? OFFSET ?`, queryArgs...)
	return list, total, err
}

func GetEarningsSummary(ctx context.Context, masterCode string) (EarningsSummary, error) {
	var totals struct {
		MonthIncome  float64 `db:"month_income"`
		TotalIncome  float64 `db:"total_income"`
		Withdrawable float64 `db:"withdrawable"`
	}
	err := earningsDB.QueryRowCtx(ctx, &totals, `SELECT
		COALESCE(SUM(CASE WHEN DATE_FORMAT(earning_date,'%Y-%m')=DATE_FORMAT(CURDATE(),'%Y-%m') THEN amount ELSE 0 END),0) month_income,
		COALESCE(SUM(amount),0) total_income,
		COALESCE(SUM(CASE WHEN settle_status=? THEN amount ELSE 0 END),0) withdrawable
		FROM master_earning WHERE master_code=?`, EarningsSettleSettled, masterCode)
	if err != nil {
		return EarningsSummary{}, err
	}

	var rows []EarningsTrendRow
	err = earningsDB.QueryRowsCtx(ctx, &rows, `SELECT DATE_FORMAT(earning_date,'%Y-%m') month,COALESCE(SUM(amount),0) amount
		FROM master_earning WHERE master_code=? AND earning_date>=DATE_FORMAT(DATE_SUB(CURDATE(),INTERVAL 5 MONTH),'%Y-%m-01')
		GROUP BY DATE_FORMAT(earning_date,'%Y-%m')`, masterCode)
	if err != nil {
		return EarningsSummary{}, err
	}
	amountByMonth := make(map[string]float64, len(rows))
	for _, row := range rows {
		amountByMonth[row.Month] = row.Amount
	}
	trend := make([]EarningsTrendRow, 0, 6)
	now := time.Now()
	for i := 5; i >= 0; i-- {
		month := now.AddDate(0, -i, 0).Format("2006-01")
		trend = append(trend, EarningsTrendRow{Month: month, Amount: amountByMonth[month]})
	}
	return EarningsSummary{
		MonthIncome: totals.MonthIncome, TotalIncome: totals.TotalIncome,
		Withdrawable: totals.Withdrawable, Trend: trend,
	}, nil
}
