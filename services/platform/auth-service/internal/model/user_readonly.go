package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const userReadonlyTable = "user"

// User 用户实体（只读，仅用于登录校验）
type User struct {
	Id       int64  `db:"id"`
	Mobile   string `db:"mobile"`
	Password string `db:"password"`
	Nickname string `db:"nickname"`
	Avatar   string `db:"avatar"`
	Gender   string `db:"gender"`
	Status   int    `db:"status"`
}

// UserReadonlyModel 用户只读模型接口
type UserReadonlyModel interface {
	FindByMobile(ctx context.Context, mobile string) (*User, error)
}

type defaultUserReadonlyModel struct {
	conn sqlx.SqlConn
}

// NewUserReadonlyModel 构造用户只读模型
func NewUserReadonlyModel(conn sqlx.SqlConn) UserReadonlyModel {
	return &defaultUserReadonlyModel{conn: conn}
}

// FindByMobile 按手机号查询用户（仅用于登录校验）
func (m *defaultUserReadonlyModel) FindByMobile(ctx context.Context, mobile string) (*User, error) {
	var u User
	query := fmt.Sprintf(`SELECT id, mobile, password, nickname, avatar, gender, status FROM %s WHERE mobile = ?`, userReadonlyTable)
	err := m.conn.QueryRowCtx(ctx, &u, query, mobile)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
