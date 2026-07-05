package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const productCategoryTable = "product_category"

// ProductCategory 商品分类表
// 对应数据库表 product_category（askxuan_product 库）
type ProductCategory struct {
	Id       int64  `db:"id" json:"id"`
	ParentId int64  `db:"parent_id" json:"parentId"`
	Name     string `db:"name" json:"name"`
	Level    int    `db:"level" json:"level"`
	Sort     int    `db:"sort" json:"sort"`
}

// ProductCategoryModel 分类模型接口
type ProductCategoryModel interface {
	Insert(ctx context.Context, data *ProductCategory) (*ProductCategory, error)
	FindOne(ctx context.Context, id int64) (*ProductCategory, error)
	Update(ctx context.Context, data *ProductCategory) error
	Delete(ctx context.Context, id int64) error
	ListByParentId(ctx context.Context, parentId int64) ([]*ProductCategory, error)
	ListAll(ctx context.Context) ([]*ProductCategory, error)
}

type defaultProductCategoryModel struct {
	conn sqlx.SqlConn
}

func NewProductCategoryModel(conn sqlx.SqlConn) ProductCategoryModel {
	return &defaultProductCategoryModel{conn: conn}
}

func (m *defaultProductCategoryModel) Insert(ctx context.Context, data *ProductCategory) (*ProductCategory, error) {
	query := fmt.Sprintf(`INSERT INTO %s (parent_id, name, level, sort) VALUES (?, ?, ?, ?)`, productCategoryTable)
	result, err := m.conn.ExecCtx(ctx, query, data.ParentId, data.Name, data.Level, data.Sort)
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

func (m *defaultProductCategoryModel) FindOne(ctx context.Context, id int64) (*ProductCategory, error) {
	var c ProductCategory
	query := fmt.Sprintf(`SELECT id, parent_id, name, level, sort FROM %s WHERE id = ?`, productCategoryTable)
	err := m.conn.QueryRowCtx(ctx, &c, query, id)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (m *defaultProductCategoryModel) Update(ctx context.Context, data *ProductCategory) error {
	query := fmt.Sprintf(`UPDATE %s SET parent_id=?, name=?, level=?, sort=? WHERE id=?`, productCategoryTable)
	_, err := m.conn.ExecCtx(ctx, query, data.ParentId, data.Name, data.Level, data.Sort, data.Id)
	return err
}

func (m *defaultProductCategoryModel) Delete(ctx context.Context, id int64) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, productCategoryTable)
	_, err := m.conn.ExecCtx(ctx, query, id)
	return err
}

func (m *defaultProductCategoryModel) ListByParentId(ctx context.Context, parentId int64) ([]*ProductCategory, error) {
	query := fmt.Sprintf(`SELECT id, parent_id, name, level, sort FROM %s WHERE parent_id = ? ORDER BY sort ASC`, productCategoryTable)
	var list []*ProductCategory
	err := m.conn.QueryRowsCtx(ctx, &list, query, parentId)
	if err != nil {
		return nil, err
	}
	return list, nil
}

func (m *defaultProductCategoryModel) ListAll(ctx context.Context) ([]*ProductCategory, error) {
	query := fmt.Sprintf(`SELECT id, parent_id, name, level, sort FROM %s ORDER BY sort ASC`, productCategoryTable)
	var list []*ProductCategory
	err := m.conn.QueryRowsCtx(ctx, &list, query)
	if err != nil {
		return nil, err
	}
	return list, nil
}
