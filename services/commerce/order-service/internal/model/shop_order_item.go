package model

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const shopOrderItemTable = "shop_order_item"

// ShopOrderItem 订单明细表
type ShopOrderItem struct {
	Id          int64   `db:"id" json:"id"`
	OrderId     int64   `db:"order_id" json:"orderId"`
	ProductId   int64   `db:"product_id" json:"productId"`
	SkuId       int64   `db:"sku_id" json:"skuId"`
	ProductName string  `db:"product_name" json:"productName"`
	SkuSpec     string  `db:"sku_spec" json:"skuSpec"`
	Price       float64 `db:"price" json:"price"`
	Quantity    int     `db:"quantity" json:"quantity"`
	Image       string  `db:"image" json:"image"`
}

// ShopOrderItemModel 订单明细接口
type ShopOrderItemModel interface {
	Insert(ctx context.Context, data *ShopOrderItem) (*ShopOrderItem, error)
	InsertWithSession(ctx context.Context, session sqlx.Session, data *ShopOrderItem) (*ShopOrderItem, error)
	ListByOrderId(ctx context.Context, orderId int64) ([]*ShopOrderItem, error)
}

type defaultShopOrderItemModel struct {
	conn sqlx.SqlConn
}

func NewShopOrderItemModel(conn sqlx.SqlConn) ShopOrderItemModel {
	return &defaultShopOrderItemModel{conn: conn}
}

func (m *defaultShopOrderItemModel) Insert(ctx context.Context, data *ShopOrderItem) (*ShopOrderItem, error) {
	return m.insert(ctx, m.conn, data)
}

func (m *defaultShopOrderItemModel) InsertWithSession(ctx context.Context, session sqlx.Session, data *ShopOrderItem) (*ShopOrderItem, error) {
	return m.insert(ctx, session, data)
}

type itemExecutor interface {
	ExecCtx(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

func (m *defaultShopOrderItemModel) insert(ctx context.Context, executor itemExecutor, data *ShopOrderItem) (*ShopOrderItem, error) {
	query := fmt.Sprintf(`INSERT INTO %s (order_id, product_id, sku_id, product_name, sku_spec, price, quantity, image) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, shopOrderItemTable)
	result, err := executor.ExecCtx(ctx, query, data.OrderId, data.ProductId, data.SkuId, data.ProductName, data.SkuSpec, data.Price, data.Quantity, data.Image)
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

func (m *defaultShopOrderItemModel) ListByOrderId(ctx context.Context, orderId int64) ([]*ShopOrderItem, error) {
	query := fmt.Sprintf(`SELECT id, order_id, product_id, sku_id, product_name, sku_spec, price, quantity, image FROM %s WHERE order_id = ?`, shopOrderItemTable)
	var list []*ShopOrderItem
	err := m.conn.QueryRowsCtx(ctx, &list, query, orderId)
	if err != nil {
		return nil, err
	}
	return list, nil
}
