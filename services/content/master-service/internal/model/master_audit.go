package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ============ 法师资质审核状态机（参照 state-machines.md 7.3） ============

// 审核状态常量
const (
	MasterAuditStatusPending  = "pending"  // 待审核
	MasterAuditStatusPass     = "pass"     // 审核通过
	MasterAuditStatusRejected = "rejected" // 审核驳回
)

// masterAuditValidTransitions 审核合法状态流转
var masterAuditValidTransitions = map[string]map[string]bool{
	MasterAuditStatusPending: {
		MasterAuditStatusPass:     true,
		MasterAuditStatusRejected: true,
	},
	MasterAuditStatusRejected: {
		MasterAuditStatusPending: true, // 修改后重新提交
	},
	// pass 为终态
}

// CanTransitMasterAudit 校验审核状态流转是否合法
func CanTransitMasterAudit(from, to string) bool {
	if from == to {
		return false
	}
	allowed, ok := masterAuditValidTransitions[from]
	if !ok {
		return false
	}
	return allowed[to]
}

// ============ 法师资质审核 MySQL 存储 ============

// MasterAudit 法师资质审核（对应 master_audit 表）
type MasterAudit struct {
	Id             int64  `db:"id" json:"id"`
	MasterCode     string `db:"master_code" json:"masterCode"`
	TempleCode     string `db:"temple_code" json:"templeCode"`
	CredentialUrls string `db:"credential_urls" json:"credentialUrls"` // JSON 数组字符串
	Status         string `db:"status" json:"status"`
	AuditorId      int64  `db:"auditor_id" json:"auditorId"`
	AuditRemark    string `db:"audit_remark" json:"auditRemark"`
	CreateTime     string `db:"create_time" json:"createTime"`
	UpdateTime     string `db:"update_time" json:"updateTime"`
}

// MasterAuditModel 法师资质审核模型接口
type MasterAuditModel interface {
	FindOne(ctx context.Context, id int64) (*MasterAudit, error)
	FindByMasterId(ctx context.Context, masterCode string) ([]*MasterAudit, error)
	InsertAuditLog(ctx context.Context, data *MasterAudit) (int64, error)
	FindAuditList(ctx context.Context, status string, page, size int) ([]*MasterAudit, int64, error)
	UpdateStatus(ctx context.Context, id int64, status string, auditorId int64, remark string) error
}

type masterAuditModel struct {
	conn  sqlx.SqlConn
	table string
}

// NewMasterAuditModel 构造法师资质审核模型
func NewMasterAuditModel(conn sqlx.SqlConn) MasterAuditModel {
	return &masterAuditModel{conn: conn, table: "master_audit"}
}

const masterAuditRows = "id, master_code, temple_code, credential_urls, status, auditor_id, audit_remark, create_time, update_time"

func (m *masterAuditModel) FindOne(ctx context.Context, id int64) (*MasterAudit, error) {
	var audit MasterAudit
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = ?", masterAuditRows, m.table)
	err := m.conn.QueryRowCtx(ctx, &audit, query, id)
	if err != nil {
		return nil, err
	}
	return &audit, nil
}

func (m *masterAuditModel) FindByMasterId(ctx context.Context, masterCode string) ([]*MasterAudit, error) {
	var list []*MasterAudit
	query := fmt.Sprintf("SELECT %s FROM %s WHERE master_code = ? ORDER BY id DESC", masterAuditRows, m.table)
	err := m.conn.QueryRowsCtx(ctx, &list, query, masterCode)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (m *masterAuditModel) InsertAuditLog(ctx context.Context, data *MasterAudit) (int64, error) {
	query := fmt.Sprintf("INSERT INTO %s (master_code, temple_code, credential_urls, status) VALUES (?, ?, ?, ?)", m.table)
	res, err := m.conn.ExecCtx(ctx, query,
		data.MasterCode, data.TempleCode, data.CredentialUrls, MasterAuditStatusPending)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (m *masterAuditModel) FindAuditList(ctx context.Context, status string, page, size int) ([]*MasterAudit, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	offset := (page - 1) * size

	where := ""
	args := []interface{}{}
	if status != "" {
		where = "WHERE status = ?"
		args = append(args, status)
	}

	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(1) FROM %s %s", m.table, where)
	err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*MasterAudit{}, 0, nil
	}

	var list []*MasterAudit
	listQuery := fmt.Sprintf("SELECT %s FROM %s %s ORDER BY id DESC LIMIT %d, %d", masterAuditRows, m.table, where, offset, size)
	err = m.conn.QueryRowsCtx(ctx, &list, listQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *masterAuditModel) UpdateStatus(ctx context.Context, id int64, status string, auditorId int64, remark string) error {
	query := fmt.Sprintf("UPDATE %s SET status = ?, auditor_id = ?, audit_remark = ? WHERE id = ?", m.table)
	_, err := m.conn.ExecCtx(ctx, query, status, auditorId, remark, id)
	return err
}
