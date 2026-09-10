package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const productSkuTable = "product_sku"

// ProductSku 商品规格表
// 对应数据库表 product_sku（askxuan_product 库）
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
	err := m.mutate(ctx, data.ProductId, func(tx sqlx.Session) error {
		result, err := tx.ExecCtx(ctx, `INSERT INTO product_sku(product_id,spec_name,spec_value,price,stock,sku_no) VALUES(?,?,?,?,?,?)`, data.ProductId, data.SpecName, data.SpecValue, data.Price, data.Stock, data.SkuNo)
		if err != nil {
			return err
		}
		data.Id, err = result.LastInsertId()
		return err
	})
	if err != nil {
		return nil, err
	}
	return data, nil
}

// Serialize admin SKU changes with cart reservation and maintain the catalog aggregate.
func (m *defaultProductSkuModel) mutate(ctx context.Context, productID int64, apply func(sqlx.Session) error) error {
	return m.conn.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var id int64
		if err := tx.QueryRowCtx(ctx, &id, `SELECT id FROM product WHERE id=? FOR UPDATE`, productID); err != nil {
			return err
		}
		if err := apply(tx); err != nil {
			return err
		}
		_, err := tx.ExecCtx(ctx, `UPDATE product SET stock=(SELECT COALESCE(SUM(stock),0) FROM product_sku WHERE product_id=?) WHERE id=?`, productID, productID)
		return err
	})
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
	old, err := m.FindOne(ctx, data.Id)
	if err != nil {
		return err
	}
	return m.mutate(ctx, old.ProductId, func(tx sqlx.Session) error {
		_, err := tx.ExecCtx(ctx, `UPDATE product_sku SET spec_name=?,spec_value=?,price=?,stock=?,sku_no=? WHERE id=?`, data.SpecName, data.SpecValue, data.Price, data.Stock, data.SkuNo, data.Id)
		return err
	})
}
func (m *defaultProductSkuModel) Delete(ctx context.Context, id int64) error {
	old, err := m.FindOne(ctx, id)
	if err != nil {
		return err
	}
	return m.mutate(ctx, old.ProductId, func(tx sqlx.Session) error {
		_, err := tx.ExecCtx(ctx, `DELETE FROM product_sku WHERE id=?`, id)
		return err
	})
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
