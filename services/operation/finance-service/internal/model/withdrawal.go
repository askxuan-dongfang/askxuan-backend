package model

import (
	"context"
	"fmt"
	"time"
)

const (
	WithdrawalPending    = "pending"
	WithdrawalApproved   = "approved"
	WithdrawalProcessing = "processing"
	WithdrawalSuccess    = "success"
	WithdrawalFailed     = "failed"
	WithdrawalRejected   = "rejected"
)

var withdrawalTransitions = map[string]map[string]bool{WithdrawalPending: {WithdrawalApproved: true, WithdrawalRejected: true}, WithdrawalApproved: {WithdrawalProcessing: true}, WithdrawalProcessing: {WithdrawalSuccess: true, WithdrawalFailed: true}, WithdrawalFailed: {WithdrawalProcessing: true}}

func CanTransitWithdrawal(from, to string) bool {
	if from == to {
		return false
	}
	allowed, ok := withdrawalTransitions[from]
	return ok && allowed[to]
}

type Withdrawal struct {
	Id            int64   `db:"id" json:"id"`
	WithdrawalNo  string  `db:"withdrawal_no" json:"withdrawalNo"`
	ApplicantType string  `db:"applicant_type" json:"applicantType"`
	ApplicantId   string  `db:"applicant_id" json:"applicantId"`
	Amount        float64 `db:"amount" json:"amount"`
	BankCard      string  `db:"bank_card" json:"bankCard"`
	Status        string  `db:"status" json:"status"`
	AuditTime     string  `db:"audit_time" json:"auditTime"`
	ProcessTime   string  `db:"process_time" json:"processTime"`
	CreateTime    string  `db:"create_time" json:"createTime"`
}

const withdrawalColumns = `id,withdrawal_no,applicant_type,applicant_id,amount,bank_card,status,IFNULL(DATE_FORMAT(audit_time,'%Y-%m-%d %H:%i:%s'),'') audit_time,IFNULL(DATE_FORMAT(process_time,'%Y-%m-%d %H:%i:%s'),'') process_time,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') create_time`

func ListWithdrawals(applicantType, status string, page, size int) ([]Withdrawal, int64) {
	where, args := "1=1", []interface{}{}
	if applicantType != "" {
		where += " AND applicant_type=?"
		args = append(args, applicantType)
	}
	if status != "" {
		where += " AND status=?"
		args = append(args, status)
	}
	var total int64
	if db.QueryRowCtx(context.Background(), &total, "SELECT COUNT(1) FROM withdrawal WHERE "+where, args...) != nil {
		return []Withdrawal{}, 0
	}
	offset, limit := pageLimit(page, size)
	var list []Withdrawal
	if db.QueryRowsCtx(context.Background(), &list, `SELECT `+withdrawalColumns+` FROM withdrawal WHERE `+where+` ORDER BY id DESC LIMIT ?,?`, append(args, offset, limit)...) != nil {
		return []Withdrawal{}, 0
	}
	return list, total
}
func FindWithdrawalByID(id int64) (Withdrawal, bool) {
	var w Withdrawal
	if db.QueryRowCtx(context.Background(), &w, `SELECT `+withdrawalColumns+` FROM withdrawal WHERE id=?`, id) != nil {
		return Withdrawal{}, false
	}
	return w, true
}
func UpdateWithdrawalStatus(id int64, status, auditTime, processTime string) bool {
	w, ok := FindWithdrawalByID(id)
	if !ok || !CanTransitWithdrawal(w.Status, status) {
		return false
	}
	res, err := db.ExecCtx(context.Background(), `UPDATE withdrawal SET status=?,audit_time=COALESCE(NULLIF(?,''),audit_time),process_time=COALESCE(NULLIF(?,''),process_time) WHERE id=? AND status=?`, status, auditTime, processTime, id, w.Status)
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	return n == 1
}
func CountWithdrawalByStatus(status string) int64 {
	var count int64
	if db.QueryRowCtx(context.Background(), &count, `SELECT COUNT(1) FROM withdrawal WHERE status=?`, status) != nil {
		return 0
	}
	return count
}
func ApplyWithdrawal(applicantType, applicantId string, amount float64, bankCard string) Withdrawal {
	no := fmt.Sprintf("WD%s%05d", time.Now().Format("20060102150405"), time.Now().UnixNano()%100000)
	res, err := db.ExecCtx(context.Background(), `INSERT INTO withdrawal(withdrawal_no,applicant_type,applicant_id,amount,bank_card,status) VALUES(?,?,?,?,?,'pending')`, no, applicantType, applicantId, amount, bankCard)
	if err != nil {
		return Withdrawal{}
	}
	id, _ := res.LastInsertId()
	return Withdrawal{Id: id, WithdrawalNo: no, ApplicantType: applicantType, ApplicantId: applicantId, Amount: amount, BankCard: bankCard, Status: WithdrawalPending, CreateTime: time.Now().Format("2006-01-02 15:04:05")}
}
