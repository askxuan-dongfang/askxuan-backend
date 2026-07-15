package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// MasterReadonlyModel 预约服务只读访问法师表（跨库查询 askxuan.master）
type MasterReadonlyModel interface {
	FindByCode(ctx context.Context, code string) (*MasterBrief, error)
	FindByID(ctx context.Context, id int64) (*MasterBrief, error)
}

// MasterBrief 法师简要信息
type MasterBrief struct {
	Id          int64  `db:"id"`
	Code        string `db:"code"`
	DharmaName  string `db:"dharma_name"`
	TempleCode  string `db:"temple_code"`
	ShelfStatus string `db:"shelf_status"`
}

type masterReadonlyModel struct {
	conn sqlx.SqlConn
}

// NewMasterReadonlyModel 构造法师只读模型
func NewMasterReadonlyModel(conn sqlx.SqlConn) MasterReadonlyModel {
	return &masterReadonlyModel{conn: conn}
}

// FindByCode 按法师编码查询（跨库 askxuan_master.master）
func (m *masterReadonlyModel) FindByCode(ctx context.Context, code string) (*MasterBrief, error) {
	var mb MasterBrief
	query := `SELECT id, code, dharma_name, temple_code, shelf_status FROM askxuan_master.master WHERE code = ?`
	err := m.conn.QueryRowCtx(ctx, &mb, query, code)
	if err != nil {
		return nil, err
	}
	return &mb, nil
}

// FindByID 按法师ID查询（跨库 askxuan_master.master）
func (m *masterReadonlyModel) FindByID(ctx context.Context, id int64) (*MasterBrief, error) {
	var mb MasterBrief
	query := `SELECT id, code, dharma_name, temple_code, shelf_status FROM askxuan_master.master WHERE id = ?`
	err := m.conn.QueryRowCtx(ctx, &mb, query, id)
	if err != nil {
		return nil, err
	}
	return &mb, nil
}
