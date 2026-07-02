package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const diyOrderItemTable = "askxuan_diy.diy_order_item"

// DiyOrderItem DIY订单明细表
type DiyOrderItem struct {
	Id           int64   `db:"id" json:"id"`
	OrderId      int64   `db:"order_id" json:"orderId"`
	MaterialId   int64   `db:"material_id" json:"materialId"`
	MaterialName string  `db:"material_name" json:"materialName"`
	Spec         string  `db:"spec" json:"spec"`
	UnitPrice    float64 `db:"unit_price" json:"unitPrice"`
	Quantity     int     `db:"quantity" json:"quantity"`
	Subtype      string  `db:"subtype" json:"subtype"`
}

// DiyOrderItemModel 订单明细接口
type DiyOrderItemModel interface {
	Insert(ctx context.Context, data *DiyOrderItem) (*DiyOrderItem, error)
	ListByOrderId(ctx context.Context, orderId int64) ([]*DiyOrderItem, error)
}

type defaultDiyOrderItemModel struct {
	conn sqlx.SqlConn
}

func NewDiyOrderItemModel(conn sqlx.SqlConn) DiyOrderItemModel {
	return &defaultDiyOrderItemModel{conn: conn}
}

func (m *defaultDiyOrderItemModel) Insert(ctx context.Context, data *DiyOrderItem) (*DiyOrderItem, error) {
	query := fmt.Sprintf(`INSERT INTO %s (order_id, material_id, material_name, spec, unit_price, quantity, subtype) VALUES (?, ?, ?, ?, ?, ?, ?)`, diyOrderItemTable)
	result, err := m.conn.ExecCtx(ctx, query, data.OrderId, data.MaterialId, data.MaterialName, data.Spec, data.UnitPrice, data.Quantity, data.Subtype)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	data.Id = id
	return data, nil
}

func (m *defaultDiyOrderItemModel) ListByOrderId(ctx context.Context, orderId int64) ([]*DiyOrderItem, error) {
	query := fmt.Sprintf(`SELECT id, order_id, material_id, material_name, spec, unit_price, quantity, subtype FROM %s WHERE order_id = ?`, diyOrderItemTable)
	var list []*DiyOrderItem
	err := m.conn.QueryRowsCtx(ctx, &list, query, orderId)
	if err != nil {
		return nil, err
	}
	return list, nil
}
