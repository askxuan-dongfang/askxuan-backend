package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ============ 角色编码常量（与 API规范 4.1 角色定义对齐） ============

const (
	RoleIdPlatformSuper   = int64(1)
	RoleIdTempleAdmin     = int64(2)
	RoleIdMaster          = int64(3)
	RoleIdShopAdmin       = int64(4)
	RoleIdPlatformService = int64(5)
)

const (
	RoleCodeCustomer        = "customer"
	RoleCodeTempleAdmin     = "temple_admin"
	RoleCodeMaster          = "master"
	RoleCodeShopAdmin       = "shop_admin"
	RoleCodePlatformSuper   = "platform_super"
	RoleCodePlatformService = "platform_service"
)

const (
	roleTable       = "askxuan_auth.role"
	permissionTable = "askxuan_auth.permission"
)

// Role 角色实体（依据 init.sql role 表）
type Role struct {
	Id          int64  `db:"id" json:"id"`
	Name        string `db:"name" json:"name"`
	Code        string `db:"code" json:"code"`
	Description string `db:"description" json:"description"`
	CreateTime  string `db:"create_time" json:"createTime"`
}

// Permission 权限实体（依据 init.sql permission 表）
type Permission struct {
	Id       int64  `db:"id" json:"id"`
	Code     string `db:"code" json:"code"`
	Name     string `db:"name" json:"name"`
	Resource string `db:"resource" json:"resource"`
	Action   string `db:"action" json:"action"`
}

// RoleModel 角色模型接口
type RoleModel interface {
	AllRoles(ctx context.Context) ([]*Role, error)
	FindByID(ctx context.Context, id int64) (*Role, error)
	Insert(ctx context.Context, data *Role) (int64, error)
	Update(ctx context.Context, data *Role) error
}

// PermissionModel 权限模型接口
type PermissionModel interface {
	AllPermissions(ctx context.Context) ([]*Permission, error)
}

type defaultRoleModel struct {
	conn sqlx.SqlConn
}

type defaultPermissionModel struct {
	conn sqlx.SqlConn
}

// NewRoleModel 构造角色模型
func NewRoleModel(conn sqlx.SqlConn) RoleModel {
	return &defaultRoleModel{conn: conn}
}

// NewPermissionModel 构造权限模型
func NewPermissionModel(conn sqlx.SqlConn) PermissionModel {
	return &defaultPermissionModel{conn: conn}
}

// AllRoles 返回全部角色
func (m *defaultRoleModel) AllRoles(ctx context.Context) ([]*Role, error) {
	query := fmt.Sprintf(`SELECT id, name, code, description, create_time FROM %s ORDER BY id ASC`, roleTable)
	var list []*Role
	if err := m.conn.QueryRowsCtx(ctx, &list, query); err != nil {
		return nil, err
	}
	return list, nil
}

// FindByID 按主键查询角色
func (m *defaultRoleModel) FindByID(ctx context.Context, id int64) (*Role, error) {
	var r Role
	query := fmt.Sprintf(`SELECT id, name, code, description, create_time FROM %s WHERE id = ?`, roleTable)
	err := m.conn.QueryRowCtx(ctx, &r, query, id)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// Insert 新建角色，返回自增 ID
func (m *defaultRoleModel) Insert(ctx context.Context, data *Role) (int64, error) {
	query := fmt.Sprintf(`INSERT INTO %s (name, code, description) VALUES (?, ?, ?)`, roleTable)
	res, err := m.conn.ExecCtx(ctx, query, data.Name, data.Code, data.Description)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// Update 更新角色（仅更新可变字段）
func (m *defaultRoleModel) Update(ctx context.Context, data *Role) error {
	query := fmt.Sprintf(`UPDATE %s SET name = ?, description = ? WHERE id = ?`, roleTable)
	_, err := m.conn.ExecCtx(ctx, query, data.Name, data.Description, data.Id)
	return err
}

// AllPermissions 返回全部权限
func (m *defaultPermissionModel) AllPermissions(ctx context.Context) ([]*Permission, error) {
	query := fmt.Sprintf(`SELECT id, code, name, resource, action FROM %s ORDER BY id ASC`, permissionTable)
	var list []*Permission
	if err := m.conn.QueryRowsCtx(ctx, &list, query); err != nil {
		return nil, err
	}
	return list, nil
}
