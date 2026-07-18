package model

import "context"

const (
	SettlementPending   = "pending"
	SettlementConfirmed = "confirmed"
	SettlementPaid      = "paid"
	SettleTypeTemple    = "temple"
	SettleTypeMaster    = "master"
	SettleTypeShop      = "shop"
)

var settlementTransitions = map[string]map[string]bool{SettlementPending: {SettlementConfirmed: true}, SettlementConfirmed: {SettlementPaid: true}}

func CanTransitSettlement(from, to string) bool {
	if from == to {
		return false
	}
	allowed, ok := settlementTransitions[from]
	return ok && allowed[to]
}

type Settlement struct {
	Id               int64   `db:"id" json:"id"`
	SettlementNo     string  `db:"settlement_no" json:"settlementNo"`
	SettleType       string  `db:"settle_type" json:"settleType"`
	TargetId         string  `db:"target_id" json:"targetId"`
	TargetName       string  `db:"target_name" json:"targetName"`
	PeriodStart      string  `db:"period_start" json:"periodStart"`
	PeriodEnd        string  `db:"period_end" json:"periodEnd"`
	OrderCount       int     `db:"order_count" json:"orderCount"`
	TotalAmount      float64 `db:"total_amount" json:"totalAmount"`
	CommissionRate   float64 `db:"commission_rate" json:"commissionRate"`
	CommissionAmount float64 `db:"commission_amount" json:"commissionAmount"`
	SettleAmount     float64 `db:"settle_amount" json:"settleAmount"`
	Status           string  `db:"status" json:"status"`
	CreateTime       string  `db:"create_time" json:"createTime"`
}

const settlementColumns = `id,settlement_no,settle_type,target_id,target_name,IFNULL(DATE_FORMAT(period_start,'%Y-%m-%d %H:%i:%s'),'') period_start,IFNULL(DATE_FORMAT(period_end,'%Y-%m-%d %H:%i:%s'),'') period_end,order_count,total_amount,commission_rate,commission_amount,settle_amount,status,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') create_time`

func ListSettlements(settleType, status string, page, size int) ([]Settlement, int64) {
	where, args := "1=1", []interface{}{}
	if settleType != "" {
		where += " AND settle_type=?"
		args = append(args, settleType)
	}
	if status != "" {
		where += " AND status=?"
		args = append(args, status)
	}
	var total int64
	if db.QueryRowCtx(context.Background(), &total, "SELECT COUNT(1) FROM settlement WHERE "+where, args...) != nil {
		return []Settlement{}, 0
	}
	offset, limit := pageLimit(page, size)
	var list []Settlement
	if db.QueryRowsCtx(context.Background(), &list, `SELECT `+settlementColumns+` FROM settlement WHERE `+where+` ORDER BY id DESC LIMIT ?,?`, append(args, offset, limit)...) != nil {
		return []Settlement{}, 0
	}
	return list, total
}
func FindSettlementByID(id int64) (Settlement, bool) {
	var s Settlement
	if db.QueryRowCtx(context.Background(), &s, `SELECT `+settlementColumns+` FROM settlement WHERE id=?`, id) != nil {
		return Settlement{}, false
	}
	return s, true
}
func UpdateSettlementStatus(id int64, status string) bool {
	s, ok := FindSettlementByID(id)
	if !ok || !CanTransitSettlement(s.Status, status) {
		return false
	}
	res, err := db.ExecCtx(context.Background(), `UPDATE settlement SET status=? WHERE id=? AND status=?`, status, id, s.Status)
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	return n == 1
}
func SumSettlementBySettleType(start, end string) map[string]float64 {
	where, args := "1=1", []interface{}{}
	if start != "" {
		where += " AND create_time>=?"
		args = append(args, start)
	}
	if end != "" {
		where += " AND create_time<=?"
		args = append(args, end)
	}
	var rows []struct {
		SettleType string  `db:"settle_type"`
		Amount     float64 `db:"amount"`
	}
	if db.QueryRowsCtx(context.Background(), &rows, `SELECT settle_type,COALESCE(SUM(total_amount),0) amount FROM settlement WHERE `+where+` GROUP BY settle_type`, args...) != nil {
		return map[string]float64{}
	}
	out := map[string]float64{}
	for _, r := range rows {
		out[r.SettleType] = r.Amount
	}
	return out
}
func SumCommissionAmount() float64 {
	var total float64
	if db.QueryRowCtx(context.Background(), &total, `SELECT COALESCE(SUM(commission_amount),0) FROM settlement`) != nil {
		return 0
	}
	return total
}
