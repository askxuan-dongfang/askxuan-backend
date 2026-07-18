package model

import "context"

type AuditLog struct {
	Id         int64  `db:"id" json:"id"`
	AuditId    int64  `db:"audit_id" json:"auditId"`
	Action     string `db:"action" json:"action"`
	OperatorId string `db:"operator_id" json:"operatorId"`
	Remark     string `db:"remark" json:"remark"`
	CreateTime string `db:"create_time" json:"createTime"`
}

func InsertAuditLog(log AuditLog) AuditLog {
	res, err := db.ExecCtx(context.Background(), `INSERT INTO audit_log(audit_id,action,operator_id,remark) VALUES(?,?,?,?)`, log.AuditId, log.Action, log.OperatorId, log.Remark)
	if err != nil {
		return AuditLog{}
	}
	log.Id, _ = res.LastInsertId()
	return log
}
func ListAuditLogByAuditID(id int64) []AuditLog {
	var list []AuditLog
	if db.QueryRowsCtx(context.Background(), &list, `SELECT id,audit_id,action,operator_id,remark,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') create_time FROM audit_log WHERE audit_id=? ORDER BY id`, id) != nil {
		return []AuditLog{}
	}
	return list
}
