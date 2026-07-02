package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// 商品图片类型
const (
	ImageTypeMain   = "main"   // 主图
	ImageTypeDetail = "detail" // 详情图
)

const productImageTable = "askxuan_shop.product_image"

// ProductImage 商品图片表
// 对应数据库表 askxuan_shop.product_image
type ProductImage struct {
	Id        int64  `db:"id" json:"id"`
	ProductId int64  `db:"product_id" json:"productId"`
	ImageUrl  string `db:"image_url" json:"imageUrl"`
	Sort      int    `db:"sort" json:"sort"`
	Type      string `db:"type" json:"type"` // main/detail
}

// ProductImageModel 图片模型接口
type ProductImageModel interface {
	Insert(ctx context.Context, data *ProductImage) error
	Delete(ctx context.Context, id int64) error
	DeleteByProductId(ctx context.Context, productId int64) error
	ListByProductId(ctx context.Context, productId int64) ([]*ProductImage, error)
}

type defaultProductImageModel struct {
	conn sqlx.SqlConn
}

func NewProductImageModel(conn sqlx.SqlConn) ProductImageModel {
	return &defaultProductImageModel{conn: conn}
}

func (m *defaultProductImageModel) Insert(ctx context.Context, data *ProductImage) error {
	query := fmt.Sprintf(`INSERT INTO %s (product_id, image_url, sort, type) VALUES (?, ?, ?, ?)`, productImageTable)
	_, err := m.conn.ExecCtx(ctx, query, data.ProductId, data.ImageUrl, data.Sort, data.Type)
	return err
}

func (m *defaultProductImageModel) Delete(ctx context.Context, id int64) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, productImageTable)
	_, err := m.conn.ExecCtx(ctx, query, id)
	return err
}

func (m *defaultProductImageModel) DeleteByProductId(ctx context.Context, productId int64) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE product_id = ?`, productImageTable)
	_, err := m.conn.ExecCtx(ctx, query, productId)
	return err
}

func (m *defaultProductImageModel) ListByProductId(ctx context.Context, productId int64) ([]*ProductImage, error) {
	query := fmt.Sprintf(`SELECT id, product_id, image_url, sort, type FROM %s WHERE product_id = ? ORDER BY sort ASC`, productImageTable)
	var list []*ProductImage
	err := m.conn.QueryRowsCtx(ctx, &list, query, productId)
	if err != nil {
		return nil, err
	}
	return list, nil
}
