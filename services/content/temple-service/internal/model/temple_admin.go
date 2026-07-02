package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ============ 寺院管理员关联 MySQL 存储 ============

// 管理员角色常量
const (
	TempleAdminRoleAdmin  = "admin"
	TempleAdminRoleEditor = "editor"
)

// templeAdminTable 寺院管理员关联表（位于 askxuan_temple 库）
const templeAdminTable = "askxuan_temple.temple_admin"

// TempleAdmin 寺院管理员关联
type TempleAdmin struct {
	Id         int64  `db:"id" json:"id"`
	TempleCode string `db:"temple_code" json:"templeCode"`
	AccountId  int64  `db:"account_id" json:"accountId"`
	Role       string `db:"role" json:"role"` // admin/editor
	CreateTime string `db:"create_time" json:"createTime"`
}

// TempleAdminModel 寺院管理员模型接口
type TempleAdminModel interface {
	Insert(ctx context.Context, data *TempleAdmin) (int64, error)
	FindOne(ctx context.Context, id int64) (*TempleAdmin, error)
	FindByAccountId(ctx context.Context, accountId int64) (*TempleAdmin, error)
	FindByTempleCode(ctx context.Context, templeCode string) ([]*TempleAdmin, error)
	Update(ctx context.Context, data *TempleAdmin) error
	Delete(ctx context.Context, id int64) error
}

type defaultTempleAdminModel struct {
	conn sqlx.SqlConn
}

// NewTempleAdminModel 构造寺院管理员模型
func NewTempleAdminModel(conn sqlx.SqlConn) TempleAdminModel {
	return &defaultTempleAdminModel{conn: conn}
}

// Insert 新增寺院管理员关联，返回自增 ID
func (m *defaultTempleAdminModel) Insert(ctx context.Context, data *TempleAdmin) (int64, error) {
	query := fmt.Sprintf(`INSERT INTO %s (temple_code, account_id, role) VALUES (?, ?, ?)`, templeAdminTable)
	res, err := m.conn.ExecCtx(ctx, query, data.TempleCode, data.AccountId, data.Role)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// FindOne 按 ID 查询寺院管理员关联
func (m *defaultTempleAdminModel) FindOne(ctx context.Context, id int64) (*TempleAdmin, error) {
	var a TempleAdmin
	query := fmt.Sprintf(`SELECT id, temple_code, account_id, role, create_time FROM %s WHERE id = ?`, templeAdminTable)
	err := m.conn.QueryRowCtx(ctx, &a, query, id)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// FindByAccountId 根据管理台账号 ID 查询所属寺院
func (m *defaultTempleAdminModel) FindByAccountId(ctx context.Context, accountId int64) (*TempleAdmin, error) {
	var a TempleAdmin
	query := fmt.Sprintf(`SELECT id, temple_code, account_id, role, create_time FROM %s WHERE account_id = ?`, templeAdminTable)
	err := m.conn.QueryRowCtx(ctx, &a, query, accountId)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// FindByTempleCode 查询寺院的管理员列表
func (m *defaultTempleAdminModel) FindByTempleCode(ctx context.Context, templeCode string) ([]*TempleAdmin, error) {
	query := fmt.Sprintf(`SELECT id, temple_code, account_id, role, create_time FROM %s WHERE temple_code = ? ORDER BY id ASC`, templeAdminTable)
	var list []*TempleAdmin
	if err := m.conn.QueryRowsCtx(ctx, &list, query, templeCode); err != nil {
		return nil, err
	}
	return list, nil
}

// Update 更新寺院管理员关联
func (m *defaultTempleAdminModel) Update(ctx context.Context, data *TempleAdmin) error {
	query := fmt.Sprintf(`UPDATE %s SET temple_code = ?, account_id = ?, role = ? WHERE id = ?`, templeAdminTable)
	_, err := m.conn.ExecCtx(ctx, query, data.TempleCode, data.AccountId, data.Role, data.Id)
	return err
}

// Delete 删除寺院管理员关联
func (m *defaultTempleAdminModel) Delete(ctx context.Context, id int64) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, templeAdminTable)
	_, err := m.conn.ExecCtx(ctx, query, id)
	return err
}
