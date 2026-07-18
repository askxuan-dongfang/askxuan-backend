package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	AuditStatusPending   = "pending"
	AuditStatusApproved  = "approved"
	AuditStatusRejected  = "rejected"
	AuditStatusFirstPass = "first_pass"
	AuditStatusFinalPass = "final_pass"
	AuditStatusVerified  = "verified"
	BizTypeDesign        = "design"
	BizTypeTemple        = "temple"
	BizTypeMaster        = "master"
	BizTypeComment       = "comment"
)

var auditTransitions = map[string]map[string]bool{AuditStatusPending: {AuditStatusApproved: true, AuditStatusRejected: true, AuditStatusFirstPass: true, AuditStatusVerified: true}, AuditStatusFirstPass: {AuditStatusFinalPass: true, AuditStatusRejected: true}, AuditStatusRejected: {AuditStatusPending: true}}

func CanTransitAudit(from, to string) bool {
	if from == to {
		return false
	}
	allowed, ok := auditTransitions[from]
	return ok && allowed[to]
}
func IsAuditTerminalStatus(s string) bool {
	return s == AuditStatusApproved || s == AuditStatusFinalPass || s == AuditStatusVerified
}

type AuditQueue struct {
	Id              int64  `db:"id" json:"id"`
	BizType         string `db:"biz_type" json:"bizType"`
	BizId           string `db:"biz_id" json:"bizId"`
	SubmitterId     string `db:"submitter_id" json:"submitterId"`
	ContentSnapshot string `db:"content_snapshot" json:"contentSnapshot"`
	Status          string `db:"status" json:"status"`
	AuditorId       string `db:"auditor_id" json:"auditorId"`
	AuditTime       string `db:"audit_time" json:"auditTime"`
	AuditRemark     string `db:"audit_remark" json:"auditRemark"`
	CreateTime      string `db:"create_time" json:"createTime"`
}

const auditColumns = `id,biz_type,biz_id,submitter_id,IFNULL(content_snapshot,'') content_snapshot,status,auditor_id,IFNULL(DATE_FORMAT(audit_time,'%Y-%m-%d %H:%i:%s'),'') audit_time,audit_remark,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') create_time`

func ListAuditQueue(bizType, status string, page, size int) ([]AuditQueue, int64) {
	where, args := "1=1", []interface{}{}
	if bizType != "" {
		where += " AND biz_type=?"
		args = append(args, bizType)
	}
	if status != "" {
		where += " AND status=?"
		args = append(args, status)
	}
	var total int64
	if db.QueryRowCtx(context.Background(), &total, "SELECT COUNT(1) FROM audit_queue WHERE "+where, args...) != nil {
		return []AuditQueue{}, 0
	}
	offset, limit := paging(page, size)
	var list []AuditQueue
	if db.QueryRowsCtx(context.Background(), &list, `SELECT `+auditColumns+` FROM audit_queue WHERE `+where+` ORDER BY id DESC LIMIT ?,?`, append(args, offset, limit)...) != nil {
		return []AuditQueue{}, 0
	}
	return list, total
}
func FindAuditQueueByID(id int64) (AuditQueue, bool) {
	var a AuditQueue
	if db.QueryRowCtx(context.Background(), &a, `SELECT `+auditColumns+` FROM audit_queue WHERE id=?`, id) != nil {
		return AuditQueue{}, false
	}
	return a, true
}
func UpdateAuditQueueStatus(id int64, status, auditorId, auditTime, remark string) bool {
	res, err := db.ExecCtx(context.Background(), `UPDATE audit_queue SET status=?,auditor_id=?,audit_time=NULLIF(?,''),audit_remark=? WHERE id=?`, status, auditorId, auditTime, remark, id)
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	return n == 1
}

func TransitionAuditQueue(id int64, to, auditorId, auditTime, remark, action string) bool {
	ctx := context.Background()
	err := db.TransactCtx(ctx, func(ctx context.Context, s sqlx.Session) error {
		var current string
		if err := s.QueryRowCtx(ctx, &current, `SELECT status FROM audit_queue WHERE id=? FOR UPDATE`, id); err != nil {
			return err
		}
		if !CanTransitAudit(current, to) {
			return sqlx.ErrNotFound
		}
		if _, err := s.ExecCtx(ctx, `UPDATE audit_queue SET status=?,auditor_id=?,audit_time=NULLIF(?,''),audit_remark=? WHERE id=?`, to, auditorId, auditTime, remark, id); err != nil {
			return err
		}
		_, err := s.ExecCtx(ctx, `INSERT INTO audit_log(audit_id,action,operator_id,remark) VALUES(?,?,?,?)`, id, action, auditorId, remark)
		return err
	})
	return err == nil
}

func CountAuditStatuses(bizType string) (total, pending, approved, rejected int64) {
	where, args := "1=1", []interface{}{}
	if bizType != "" {
		where = "biz_type=?"
		args = append(args, bizType)
	}
	query := `SELECT COUNT(1) total,SUM(status='pending') pending,SUM(status IN ('approved','first_pass','final_pass','verified')) approved,SUM(status='rejected') rejected FROM audit_queue WHERE ` + where
	var row struct {
		Total    int64 `db:"total"`
		Pending  int64 `db:"pending"`
		Approved int64 `db:"approved"`
		Rejected int64 `db:"rejected"`
	}
	if db.QueryRowCtx(context.Background(), &row, query, args...) != nil {
		return
	}
	return row.Total, row.Pending, row.Approved, row.Rejected
}
