package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ============ 资质审核状态常量 ============

const (
	CredentialStatusPending  = "pending"  // 待审核
	CredentialStatusVerified = "verified" // 已通过
	CredentialStatusRejected = "rejected" // 已驳回
)

// MasterCredential 法师资质证书（对应 master_credential 表）
type MasterCredential struct {
	Id         int64  `db:"id" json:"id"`
	MasterCode string `db:"master_code" json:"masterCode"`
	CertType   string `db:"cert_type" json:"certType"` // ordination/identity/title
	CertUrl    string `db:"cert_url" json:"certUrl"`
	Status     string `db:"status" json:"status"` // pending/verified/rejected
	SubmitTime string `db:"submit_time" json:"submitTime"`
	AuditTime  string `db:"audit_time" json:"auditTime"`
}

// MasterCredentialModel 法师资质证书模型接口
type MasterCredentialModel interface {
	FindByMasterId(ctx context.Context, masterCode string) ([]*MasterCredential, error)
	Insert(ctx context.Context, data *MasterCredential) (int64, error)
	Update(ctx context.Context, data *MasterCredential) error
}

type masterCredentialModel struct {
	conn  sqlx.SqlConn
	table string
}

// NewMasterCredentialModel 构造法师资质证书模型
func NewMasterCredentialModel(conn sqlx.SqlConn) MasterCredentialModel {
	return &masterCredentialModel{conn: conn, table: "master_credential"}
}

const masterCredentialRows = "id, master_code, cert_type, cert_url, status, submit_time, audit_time"

func (m *masterCredentialModel) FindByMasterId(ctx context.Context, masterCode string) ([]*MasterCredential, error) {
	var list []*MasterCredential
	query := fmt.Sprintf("SELECT %s FROM %s WHERE master_code = ? ORDER BY id DESC", masterCredentialRows, m.table)
	err := m.conn.QueryRowsCtx(ctx, &list, query, masterCode)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (m *masterCredentialModel) Insert(ctx context.Context, data *MasterCredential) (int64, error) {
	query := fmt.Sprintf("INSERT INTO %s (master_code, cert_type, cert_url, status) VALUES (?, ?, ?, ?)", m.table)
	res, err := m.conn.ExecCtx(ctx, query,
		data.MasterCode, data.CertType, data.CertUrl, CredentialStatusPending)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (m *masterCredentialModel) Update(ctx context.Context, data *MasterCredential) error {
	query := fmt.Sprintf("UPDATE %s SET cert_type = ?, cert_url = ?, status = ? WHERE id = ?", m.table)
	_, err := m.conn.ExecCtx(ctx, query, data.CertType, data.CertUrl, data.Status, data.Id)
	return err
}
