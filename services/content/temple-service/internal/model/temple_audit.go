package model

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ============ 寺院入驻审核状态机（参照 state-machines.md 7.2） ============

// 审核状态常量
const (
	TempleAuditStatusPending   = "pending"    // 待初审
	TempleAuditStatusFirstPass = "first_pass" // 初审通过
	TempleAuditStatusFinalPass = "final_pass" // 终审通过
	TempleAuditStatusRejected  = "rejected"   // 驳回
)

// templeAuditValidTransitions 审核合法状态流转
var templeAuditValidTransitions = map[string]map[string]bool{
	TempleAuditStatusPending: {
		TempleAuditStatusFirstPass: true,
		TempleAuditStatusRejected:  true,
	},
	TempleAuditStatusFirstPass: {
		TempleAuditStatusFinalPass: true,
		TempleAuditStatusRejected:  true,
	},
	TempleAuditStatusRejected: {
		TempleAuditStatusPending: true, // 修改后重新提交
	},
	// final_pass 为终态
}

// CanTransitTempleAudit 校验审核状态流转是否合法
func CanTransitTempleAudit(from, to string) bool {
	if from == to {
		return false
	}
	allowed, ok := templeAuditValidTransitions[from]
	if !ok {
		return false
	}
	return allowed[to]
}

// ============ 寺院入驻审核 MySQL 存储 ============

// templeAuditTable 审核表（位于 askxuan_temple 库）
const templeAuditTable = "askxuan_temple.temple_audit"

// TempleAudit 寺院入驻审核
type TempleAudit struct {
	Id            int64    `db:"id" json:"id"`
	TempleCode    string   `db:"temple_code" json:"templeCode"`
	ApplicantName string   `db:"applicant_name" json:"applicantName"`
	ContactPhone  string   `db:"contact_phone" json:"contactPhone"`
	CertUrls      []string `json:"certUrls"` // DB 中为 JSON 字符串
	Status        string   `db:"status" json:"status"`
	AuditorId     int64    `db:"auditor_id" json:"auditorId"`
	AuditRemark   string   `db:"audit_remark" json:"auditRemark"`
	CreateTime    string   `db:"create_time" json:"createTime"`
	UpdateTime    string   `db:"update_time" json:"updateTime"`
}

// templeAuditRow 审核记录 DB 行结构（CertUrls 为 JSON 字符串）
type templeAuditRow struct {
	Id            int64  `db:"id"`
	TempleCode    string `db:"temple_code"`
	ApplicantName string `db:"applicant_name"`
	ContactPhone  string `db:"contact_phone"`
	CertUrls      string `db:"cert_urls"`
	Status        string `db:"status"`
	AuditorId     int64  `db:"auditor_id"`
	AuditRemark   string `db:"audit_remark"`
	CreateTime    string `db:"create_time"`
	UpdateTime    string `db:"update_time"`
}

// TempleAuditModel 寺院入驻审核模型接口
type TempleAuditModel interface {
	Insert(ctx context.Context, data *TempleAudit) (int64, error)
	FindOne(ctx context.Context, id int64) (*TempleAudit, error)
	FindByTempleId(ctx context.Context, templeCode string) ([]*TempleAudit, error)
	FindList(ctx context.Context, status string, page, size int) ([]*TempleAudit, int64, error)
	Update(ctx context.Context, data *TempleAudit) error
	UpdateStatus(ctx context.Context, id int64, status string, auditorId int64, remark string) error
	Delete(ctx context.Context, id int64) error
}

type defaultTempleAuditModel struct {
	conn sqlx.SqlConn
}

// NewTempleAuditModel 构造审核模型
func NewTempleAuditModel(conn sqlx.SqlConn) TempleAuditModel {
	return &defaultTempleAuditModel{conn: conn}
}

// Insert 提交入驻申请（status=pending），返回自增 ID
func (m *defaultTempleAuditModel) Insert(ctx context.Context, data *TempleAudit) (int64, error) {
	certUrlsJSON := "[]"
	if len(data.CertUrls) > 0 {
		if b, err := json.Marshal(data.CertUrls); err == nil {
			certUrlsJSON = string(b)
		}
	}
	if data.Status == "" {
		data.Status = TempleAuditStatusPending
	}
	query := fmt.Sprintf(`INSERT INTO %s (temple_code, applicant_name, contact_phone, cert_urls, status, auditor_id, audit_remark) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		templeAuditTable)
	res, err := m.conn.ExecCtx(ctx, query,
		data.TempleCode, data.ApplicantName, data.ContactPhone,
		certUrlsJSON, data.Status, data.AuditorId, data.AuditRemark)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// FindOne 按 ID 查询审核记录
func (m *defaultTempleAuditModel) FindOne(ctx context.Context, id int64) (*TempleAudit, error) {
	query := fmt.Sprintf(`SELECT id, temple_code, applicant_name, contact_phone, cert_urls, status, auditor_id, audit_remark, create_time, update_time FROM %s WHERE id = ?`,
		templeAuditTable)
	var row templeAuditRow
	if err := m.conn.QueryRowCtx(ctx, &row, query, id); err != nil {
		return nil, err
	}
	return rowToAudit(&row), nil
}

// FindByTempleId 查询寺院的审核记录
func (m *defaultTempleAuditModel) FindByTempleId(ctx context.Context, templeCode string) ([]*TempleAudit, error) {
	query := fmt.Sprintf(`SELECT id, temple_code, applicant_name, contact_phone, cert_urls, status, auditor_id, audit_remark, create_time, update_time FROM %s WHERE temple_code = ? ORDER BY id DESC`,
		templeAuditTable)
	var rows []*templeAuditRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, templeCode); err != nil {
		return nil, err
	}
	list := make([]*TempleAudit, 0, len(rows))
	for _, r := range rows {
		list = append(list, rowToAudit(r))
	}
	return list, nil
}

// FindList 查询审核列表，支持按 status 筛选 + 分页
func (m *defaultTempleAuditModel) FindList(ctx context.Context, status string, page, size int) ([]*TempleAudit, int64, error) {
	where := "1=1"
	var args []interface{}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(1) FROM %s WHERE %s`, templeAuditTable, where)
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*TempleAudit{}, 0, nil
	}

	offset := (page - 1) * size
	listQuery := fmt.Sprintf(`SELECT id, temple_code, applicant_name, contact_phone, cert_urls, status, auditor_id, audit_remark, create_time, update_time FROM %s WHERE %s ORDER BY id DESC LIMIT ?, ?`,
		templeAuditTable, where)
	listArgs := append(args, offset, size)
	var rows []*templeAuditRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, listQuery, listArgs...); err != nil {
		return nil, 0, err
	}
	list := make([]*TempleAudit, 0, len(rows))
	for _, r := range rows {
		list = append(list, rowToAudit(r))
	}
	return list, total, nil
}

// Update 更新审核记录
func (m *defaultTempleAuditModel) Update(ctx context.Context, data *TempleAudit) error {
	certUrlsJSON := "[]"
	if len(data.CertUrls) > 0 {
		if b, err := json.Marshal(data.CertUrls); err == nil {
			certUrlsJSON = string(b)
		}
	}
	query := fmt.Sprintf(`UPDATE %s SET temple_code = ?, applicant_name = ?, contact_phone = ?, cert_urls = ?, status = ?, auditor_id = ?, audit_remark = ? WHERE id = ?`,
		templeAuditTable)
	_, err := m.conn.ExecCtx(ctx, query,
		data.TempleCode, data.ApplicantName, data.ContactPhone,
		certUrlsJSON, data.Status, data.AuditorId, data.AuditRemark, data.Id)
	return err
}

// UpdateStatus 更新审核状态（调用方需先校验 CanTransitTempleAudit）
func (m *defaultTempleAuditModel) UpdateStatus(ctx context.Context, id int64, status string, auditorId int64, remark string) error {
	query := fmt.Sprintf(`UPDATE %s SET status = ?, auditor_id = ?`, templeAuditTable)
	args := []interface{}{status, auditorId}
	if remark != "" {
		query += `, audit_remark = ?`
		args = append(args, remark)
	}
	query += ` WHERE id = ?`
	args = append(args, id)
	_, err := m.conn.ExecCtx(ctx, query, args...)
	return err
}

// Delete 删除审核记录
func (m *defaultTempleAuditModel) Delete(ctx context.Context, id int64) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, templeAuditTable)
	_, err := m.conn.ExecCtx(ctx, query, id)
	return err
}

// rowToAudit 将 DB 行结构转为 TempleAudit（CertUrls JSON 反序列化）
func rowToAudit(row *templeAuditRow) *TempleAudit {
	var certUrls []string
	if row.CertUrls != "" {
		_ = json.Unmarshal([]byte(row.CertUrls), &certUrls)
	}
	if certUrls == nil {
		certUrls = []string{}
	}
	return &TempleAudit{
		Id:            row.Id,
		TempleCode:    row.TempleCode,
		ApplicantName: row.ApplicantName,
		ContactPhone:  row.ContactPhone,
		CertUrls:      certUrls,
		Status:        row.Status,
		AuditorId:     row.AuditorId,
		AuditRemark:   row.AuditRemark,
		CreateTime:    row.CreateTime,
		UpdateTime:    row.UpdateTime,
	}
}
