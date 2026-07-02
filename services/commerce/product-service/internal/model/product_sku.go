package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const productSkuTable = "askxuan_shop.product_sku"

// ProductSku 商品规格表
// 对应数据库表 askxuan_shop.product_sku
type ProductSku struct {
	Id        int64   `db:"id" json:"id"`
	ProductId int64   `db:"product_id" json:"productId"`
	SpecName  string  `db:"spec_name" json:"specName"`
	SpecValue string  `db:"spec_value" json:"specValue"`
	Price     float64 `db:"price" json:"price"`
	Stock     int     `db:"stock" json:"stock"`
	SkuNo     string  `db:"sku_no" json:"skuNo"`
}

// ProductSkuModel SKU 模型接口
type ProductSkuModel interface {
	Insert(ctx context.Context, data *ProductSku) (*ProductSku, error)
	FindOne(ctx context.Context, id int64) (*ProductSku, error)
	Update(ctx context.Context, data *ProductSku) error
	Delete(ctx context.Context, id int64) error
	ListByProductId(ctx context.Context, productId int64) ([]*ProductSku, error)
}

type defaultProductSkuModel struct {
	conn sqlx.SqlConn
}

func NewProductSkuModel(conn sqlx.SqlConn) ProductSkuModel {
	return &defaultProductSkuModel{conn: conn}
}

func (m *defaultProductSkuModel) Insert(ctx context.Context, data *ProductSku) (*ProductSku, error) {
	query := fmt.Sprintf(`INSERT INTO %s (product_id, spec_name, spec_value, price, stock, sku_no) VALUES (?, ?, ?, ?, ?, ?)`, productSkuTable)
	result, err := m.conn.ExecCtx(ctx, query, data.ProductId, data.SpecName, data.SpecValue, data.Price, data.Stock, data.SkuNo)
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

func (m *defaultProductSkuModel) FindOne(ctx context.Context, id int64) (*ProductSku, error) {
	var s ProductSku
	query := fmt.Sprintf(`SELECT id, product_id, spec_name, spec_value, price, stock, sku_no FROM %s WHERE id = ?`, productSkuTable)
	err := m.conn.QueryRowCtx(ctx, &s, query, id)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (m *defaultProductSkuModel) Update(ctx context.Context, data *ProductSku) error {
	query := fmt.Sprintf(`UPDATE %s SET spec_name=?, spec_value=?, price=?, stock=?, sku_no=? WHERE id=?`, productSkuTable)
	_, err := m.conn.ExecCtx(ctx, query, data.SpecName, data.SpecValue, data.Price, data.Stock, data.SkuNo, data.Id)
	return err
}

func (m *defaultProductSkuModel) Delete(ctx context.Context, id int64) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, productSkuTable)
	_, err := m.conn.ExecCtx(ctx, query, id)
	return err
}

func (m *defaultProductSkuModel) ListByProductId(ctx context.Context, productId int64) ([]*ProductSku, error) {
	query := fmt.Sprintf(`SELECT id, product_id, spec_name, spec_value, price, stock, sku_no FROM %s WHERE product_id = ?`, productSkuTable)
	var list []*ProductSku
	err := m.conn.QueryRowsCtx(ctx, &list, query, productId)
	if err != nil {
		return nil, err
	}
	return list, nil
}
