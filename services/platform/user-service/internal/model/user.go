package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ============ 用户状态常量 ============

const (
	UserStatusNormal = 1 // 正常
	UserStatusBanned = 0 // 禁用
)

// userTable 用户表（位于 askxuan 默认库）
const userTable = "user"

// User 用户实体（依据 init.sql user 表）
type User struct {
	Id         int64  `db:"id" json:"userId"`
	Mobile     string `db:"mobile" json:"mobile"`
	Password   string `db:"password" json:"-"`
	Nickname   string `db:"nickname" json:"nickname"`
	Avatar     string `db:"avatar" json:"avatar"`
	Gender     string `db:"gender" json:"gender"`
	Birthday   string `db:"birthday" json:"birthday"`
	Region     string `db:"region" json:"region"`
	Bio        string `db:"bio" json:"bio"`
	Status     int    `db:"status" json:"status"`
	CreateTime string `db:"create_time" json:"createTime"`
	UpdateTime string `db:"update_time" json:"updateTime"`
}

// UserFilter 用户列表查询过滤条件
type UserFilter struct {
	Keyword string // 昵称/手机号模糊匹配
	Status  int    // 0=禁用 1=正常，-1=全部
}

// UserModel 用户模型接口
type UserModel interface {
	FindByMobile(ctx context.Context, mobile string) (*User, error)
	FindByID(ctx context.Context, id int64) (*User, error)
	Insert(ctx context.Context, data *User) (int64, error)
	Update(ctx context.Context, data *User) error
	UpdateStatus(ctx context.Context, id int64, status int) error
	FindList(ctx context.Context, filter UserFilter, page, size int) ([]*User, int64, error)
}

type defaultUserModel struct {
	conn sqlx.SqlConn
}

// NewUserModel 构造用户模型
func NewUserModel(conn sqlx.SqlConn) UserModel {
	return &defaultUserModel{conn: conn}
}

// FindByMobile 按手机号查询用户
func (m *defaultUserModel) FindByMobile(ctx context.Context, mobile string) (*User, error) {
	var u User
	query := fmt.Sprintf(`SELECT id, mobile, password, nickname, avatar, gender, IFNULL(birthday,'') AS birthday, region, bio, status, create_time, update_time FROM %s WHERE mobile = ?`, userTable)
	err := m.conn.QueryRowCtx(ctx, &u, query, mobile)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// FindByID 按 userId 查询
func (m *defaultUserModel) FindByID(ctx context.Context, id int64) (*User, error) {
	var u User
	query := fmt.Sprintf(`SELECT id, mobile, password, nickname, avatar, gender, IFNULL(birthday,'') AS birthday, region, bio, status, create_time, update_time FROM %s WHERE id = ?`, userTable)
	err := m.conn.QueryRowCtx(ctx, &u, query, id)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// Insert 注册新用户
func (m *defaultUserModel) Insert(ctx context.Context, data *User) (int64, error) {
	const insertUser = `INSERT INTO ` + userTable + ` (mobile, password, nickname, avatar, gender, region, bio, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	const insertProfile = `INSERT INTO ` + userProfileTable + ` (user_id, preference_tags, total_orders, total_spent, last_active_time) VALUES (?, '', 0, 0.00, NOW())`

	var id int64
	err := m.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		res, err := session.ExecCtx(ctx, insertUser,
			data.Mobile, data.Password, data.Nickname, data.Avatar,
			data.Gender, data.Region, data.Bio, UserStatusNormal)
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		if err != nil {
			return err
		}
		_, err = session.ExecCtx(ctx, insertProfile, id)
		return err
	})
	return id, err
}

// Update 更新用户资料（仅更新非空字段）
// 注：birthday 为 DATE 类型，空字符串会报 Incorrect date value，用 NULLIF(?, ”) 转为 NULL
func (m *defaultUserModel) Update(ctx context.Context, data *User) error {
	const query = `UPDATE ` + userTable + ` SET nickname = ?, avatar = ?, gender = ?, birthday = NULLIF(?, ''), region = ?, bio = ? WHERE id = ?`
	_, err := m.conn.ExecCtx(ctx, query,
		data.Nickname, data.Avatar, data.Gender, data.Birthday,
		data.Region, data.Bio, data.Id)
	return err
}

// UpdateStatus 更新用户状态（封禁/解封）
func (m *defaultUserModel) UpdateStatus(ctx context.Context, id int64, status int) error {
	const query = `UPDATE ` + userTable + ` SET status = ? WHERE id = ?`
	_, err := m.conn.ExecCtx(ctx, query, status, id)
	return err
}

// FindList 用户列表查询，支持按 keyword（昵称/手机号）/status 筛选 + 分页
func (m *defaultUserModel) FindList(ctx context.Context, filter UserFilter, page, size int) ([]*User, int64, error) {
	where, args := buildUserWhere(filter)

	countQuery := fmt.Sprintf(`SELECT COUNT(1) FROM %s WHERE %s`, userTable, where)
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*User{}, 0, nil
	}

	offset := (page - 1) * size
	listQuery := fmt.Sprintf(`SELECT id, mobile, password, nickname, avatar, gender, IFNULL(birthday,'') AS birthday, region, bio, status, create_time, update_time FROM %s WHERE %s ORDER BY id DESC LIMIT ?, ?`,
		userTable, where)
	listArgs := append(args, offset, size)
	var list []*User
	if err := m.conn.QueryRowsCtx(ctx, &list, listQuery, listArgs...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// buildUserWhere 构建用户列表 WHERE 子句
func buildUserWhere(filter UserFilter) (string, []interface{}) {
	where := "1=1"
	var args []interface{}
	if filter.Keyword != "" {
		where += " AND (nickname LIKE ? OR mobile LIKE ?)"
		kw := "%" + filter.Keyword + "%"
		args = append(args, kw, kw)
	}
	if filter.Status >= 0 {
		where += " AND status = ?"
		args = append(args, filter.Status)
	}
	return where, args
}
