package model

import "context"

type FinanceLog struct {
	Id           int64   `db:"id" json:"id"`
	SettlementId int64   `db:"settlement_id" json:"settlementId"`
	Amount       float64 `db:"amount" json:"amount"`
	Type         string  `db:"type" json:"type"`
	Description  string  `db:"description" json:"description"`
	CreateTime   string  `db:"create_time" json:"createTime"`
}

func InsertFinanceLog(log FinanceLog) FinanceLog {
	res, err := db.ExecCtx(context.Background(), `INSERT INTO finance_log(settlement_id,amount,type,description) VALUES(?,?,?,?)`, log.SettlementId, log.Amount, log.Type, log.Description)
	if err != nil {
		return FinanceLog{}
	}
	log.Id, _ = res.LastInsertId()
	return log
}
func SumByType(start, end string) map[string]float64 {
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
		Type   string  `db:"type"`
		Amount float64 `db:"amount"`
	}
	if db.QueryRowsCtx(context.Background(), &rows, `SELECT type,COALESCE(SUM(amount),0) amount FROM finance_log WHERE `+where+` GROUP BY type`, args...) != nil {
		return map[string]float64{}
	}
	out := map[string]float64{}
	for _, r := range rows {
		out[r.Type] = r.Amount
	}
	return out
}
