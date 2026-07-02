package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// TempleReadonlyModel 预约服务只读访问寺院表（跨库查询 askxuan.temple）
type TempleReadonlyModel interface {
	FindByCode(ctx context.Context, code string) (*TempleBrief, error)
}

// TempleBrief 寺寺简要信息
type TempleBrief struct {
	Id     int64  `db:"id"`
	Code   string `db:"code"`
	Name   string `db:"name"`
	Status string `db:"status"`
}

type templeReadonlyModel struct {
	conn sqlx.SqlConn
}

// NewTempleReadonlyModel 构造寺院只读模型
func NewTempleReadonlyModel(conn sqlx.SqlConn) TempleReadonlyModel {
	return &templeReadonlyModel{conn: conn}
}

// FindByCode 按寺院编码查询（跨库 askxuan.temple）
func (m *templeReadonlyModel) FindByCode(ctx context.Context, code string) (*TempleBrief, error) {
	var t TempleBrief
	query := `SELECT id, code, name, status FROM askxuan.temple WHERE code = ?`
	err := m.conn.QueryRowCtx(ctx, &t, query, code)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
