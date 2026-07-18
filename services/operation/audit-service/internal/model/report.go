package model

import "context"

const (
	ReportStatusPending  = "pending"
	ReportStatusHandled  = "handled"
	ReportStatusRejected = "rejected"
	ReportTargetDesign   = "design"
	ReportTargetComment  = "comment"
	ReportTargetMaster   = "master"
	ReportTargetTemple   = "temple"
)

var reportTransitions = map[string]map[string]bool{ReportStatusPending: {ReportStatusHandled: true, ReportStatusRejected: true}}

func CanTransitReport(from, to string) bool {
	if from == to {
		return false
	}
	allowed, ok := reportTransitions[from]
	return ok && allowed[to]
}

type Report struct {
	Id           int64  `db:"id" json:"id"`
	ReporterId   string `db:"reporter_id" json:"reporterId"`
	TargetType   string `db:"target_type" json:"targetType"`
	TargetId     string `db:"target_id" json:"targetId"`
	Reason       string `db:"reason" json:"reason"`
	EvidenceUrls string `db:"evidence_urls" json:"evidenceUrls"`
	Status       string `db:"status" json:"status"`
	HandlerId    string `db:"handler_id" json:"handlerId"`
	HandleResult string `db:"handle_result" json:"handleResult"`
	CreateTime   string `db:"create_time" json:"createTime"`
}

const reportColumns = `id,reporter_id,target_type,target_id,reason,IFNULL(evidence_urls,'') evidence_urls,status,handler_id,handle_result,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') create_time`

func ListReports(targetType, status string, page, size int) ([]Report, int64) {
	where, args := "1=1", []interface{}{}
	if targetType != "" {
		where += " AND target_type=?"
		args = append(args, targetType)
	}
	if status != "" {
		where += " AND status=?"
		args = append(args, status)
	}
	var total int64
	if db.QueryRowCtx(context.Background(), &total, "SELECT COUNT(1) FROM report WHERE "+where, args...) != nil {
		return []Report{}, 0
	}
	offset, limit := paging(page, size)
	var list []Report
	if db.QueryRowsCtx(context.Background(), &list, `SELECT `+reportColumns+` FROM report WHERE `+where+` ORDER BY id DESC LIMIT ?,?`, append(args, offset, limit)...) != nil {
		return []Report{}, 0
	}
	return list, total
}
func FindReportByID(id int64) (Report, bool) {
	var r Report
	if db.QueryRowCtx(context.Background(), &r, `SELECT `+reportColumns+` FROM report WHERE id=?`, id) != nil {
		return Report{}, false
	}
	return r, true
}
func UpdateReport(id int64, status, handlerId, handleResult string) bool {
	res, err := db.ExecCtx(context.Background(), `UPDATE report SET status=?,handler_id=?,handle_result=? WHERE id=? AND status='pending'`, status, handlerId, handleResult, id)
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	return n == 1
}
