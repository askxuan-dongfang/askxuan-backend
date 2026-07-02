package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// 账号状态常量
const (
	AccountStatusEnabled  = "enabled"
	AccountStatusDisabled = "disabled"
)

// adminAccountTable 管理台账号表（跨库查询 askxuan_auth.admin_account）
const adminAccountTable = "askxuan_auth.admin_account"

// AdminAccount 管理台账号实体（依据 init.sql admin_account 表）
type AdminAccount struct {
	Id            int64  `db:"id" json:"id"`
	Account       string `db:"account" json:"account"`
	Password      string `db:"password" json:"-"`
	Name          string `db:"name" json:"name"`
	RoleId        int64  `db:"role_id" json:"roleId"`
	TempleId      string `db:"temple_id" json:"templeId"`
	MasterId      string `db:"master_id" json:"masterId"`
	ShopId        int64  `db:"shop_id" json:"shopId"`
	Status        string `db:"status" json:"status"`
	LastLoginTime string `db:"last_login_time" json:"lastLoginTime"`
	CreateTime    string `db:"create_time" json:"createTime"`
	UpdateTime    string `db:"update_time" json:"updateTime"`
}

// AdminAccountModel 管理台账号模型接口
type AdminAccountModel interface {
	FindByAccount(ctx context.Context, account string) (*AdminAccount, error)
	FindByID(ctx context.Context, id int64) (*AdminAccount, error)
	FindList(ctx context.Context, keyword, status string, page, size int) ([]*AdminAccount, int64, error)
	Insert(ctx context.Context, data *AdminAccount) (int64, error)
	Update(ctx context.Context, data *AdminAccount) error
	UpdateStatus(ctx context.Context, id int64, status string) error
	UpdateLastLogin(ctx context.Context, id int64) error
}

type defaultAdminAccountModel struct {
	conn sqlx.SqlConn
}

// NewAdminAccountModel 构造管理台账号模型
func NewAdminAccountModel(conn sqlx.SqlConn) AdminAccountModel {
	return &defaultAdminAccountModel{conn: conn}
}

// adminAccountColumns 查询列，使用 IFNULL 处理可能为 NULL 的字段
const adminAccountColumns = `id, account, password, name, role_id, temple_id, master_id, shop_id, status, IFNULL(last_login_time,'') AS last_login_time, create_time, update_time`

// FindByAccount 按登录账号查询
func (m *defaultAdminAccountModel) FindByAccount(ctx context.Context, account string) (*AdminAccount, error) {
	var a AdminAccount
	query := fmt.Sprintf(`SELECT %s FROM %s WHERE account = ?`, adminAccountColumns, adminAccountTable)
	err := m.conn.QueryRowCtx(ctx, &a, query, account)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// FindByID 按主键查询
func (m *defaultAdminAccountModel) FindByID(ctx context.Context, id int64) (*AdminAccount, error) {
	var a AdminAccount
	query := fmt.Sprintf(`SELECT %s FROM %s WHERE id = ?`, adminAccountColumns, adminAccountTable)
	err := m.conn.QueryRowCtx(ctx, &a, query, id)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// FindList 账号列表查询，支持 keyword（account/name 模糊搜索）+ status 筛选 + 分页
func (m *defaultAdminAccountModel) FindList(ctx context.Context, keyword, status string, page, size int) ([]*AdminAccount, int64, error) {
	where, args := buildAdminAccountWhere(keyword, status)

	countQuery := fmt.Sprintf(`SELECT COUNT(1) FROM %s WHERE %s`, adminAccountTable, where)
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*AdminAccount{}, 0, nil
	}

	offset := (page - 1) * size
	listQuery := fmt.Sprintf(`SELECT %s FROM %s WHERE %s ORDER BY id ASC LIMIT ?, ?`,
		adminAccountColumns, adminAccountTable, where)
	listArgs := append(args, offset, size)
	var list []*AdminAccount
	if err := m.conn.QueryRowsCtx(ctx, &list, listQuery, listArgs...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// Insert 新建账号，返回自增 ID
func (m *defaultAdminAccountModel) Insert(ctx context.Context, data *AdminAccount) (int64, error) {
	query := fmt.Sprintf(`INSERT INTO %s (account, password, name, role_id, temple_id, master_id, shop_id, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, adminAccountTable)
	res, err := m.conn.ExecCtx(ctx, query,
		data.Account, data.Password, data.Name, data.RoleId,
		data.TempleId, data.MasterId, data.ShopId, AccountStatusEnabled)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// Update 更新账号（仅更新可变字段）
func (m *defaultAdminAccountModel) Update(ctx context.Context, data *AdminAccount) error {
	query := fmt.Sprintf(`UPDATE %s SET name = ?, role_id = ?, temple_id = ?, master_id = ?, shop_id = ? WHERE id = ?`, adminAccountTable)
	_, err := m.conn.ExecCtx(ctx, query,
		data.Name, data.RoleId, data.TempleId, data.MasterId, data.ShopId, data.Id)
	return err
}

// UpdateStatus 更新账号状态（启用/禁用）
func (m *defaultAdminAccountModel) UpdateStatus(ctx context.Context, id int64, status string) error {
	query := fmt.Sprintf(`UPDATE %s SET status = ? WHERE id = ?`, adminAccountTable)
	_, err := m.conn.ExecCtx(ctx, query, status, id)
	return err
}

// UpdateLastLogin 更新最后登录时间为当前时间
func (m *defaultAdminAccountModel) UpdateLastLogin(ctx context.Context, id int64) error {
	query := fmt.Sprintf(`UPDATE %s SET last_login_time = NOW() WHERE id = ?`, adminAccountTable)
	_, err := m.conn.ExecCtx(ctx, query, id)
	return err
}

// buildAdminAccountWhere 构建账号列表 WHERE 子句
func buildAdminAccountWhere(keyword, status string) (string, []interface{}) {
	where := "1=1"
	var args []interface{}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	if keyword != "" {
		where += " AND (account LIKE ? OR name LIKE ?)"
		kw := "%" + keyword + "%"
		args = append(args, kw, kw)
	}
	return where, args
}
