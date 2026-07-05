package model

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// 商品状态常量
const (
	ProductStatusDraft    = "draft"    // 草稿
	ProductStatusOnShelf  = "on_shelf" // 上架
	ProductStatusOffShelf = "off_shelf" // 下架
)

const productTable = "product"

// Product 商品表
// 对应数据库表 product（askxuan_product 库）
type Product struct {
	Id                int64   `db:"id" json:"id"`
	ProductNo         string  `db:"product_no" json:"productNo"`
	Name              string  `db:"name" json:"name"`
	CategoryId        int64   `db:"category_id" json:"categoryId"`
	Description       string  `db:"description" json:"description"`
	MainImage         string  `db:"main_image" json:"mainImage"`
	Status            string  `db:"status" json:"status"` // draft/on_shelf/off_shelf
	Price             float64 `db:"price" json:"price"`
	MarketPrice       float64 `db:"market_price" json:"marketPrice"`
	Stock             int     `db:"stock" json:"stock"`
	Tags              string  `db:"tags" json:"tags"`
	FreightTemplateId int64   `db:"freight_template_id" json:"freightTemplateId"`
	CreateTime        string  `db:"create_time" json:"createTime"`
	UpdateTime        string  `db:"update_time" json:"updateTime"`
}

// ProductModel 商品模型接口
type ProductModel interface {
	Insert(ctx context.Context, data *Product) (*Product, error)
	FindOne(ctx context.Context, id int64) (*Product, error)
	FindList(ctx context.Context, categoryId int64, keyword, status string, page, size int) ([]*Product, int64, error)
	Update(ctx context.Context, data *Product) error
	UpdateStatus(ctx context.Context, id int64, status string) error
	Delete(ctx context.Context, id int64) error
}

type defaultProductModel struct {
	conn sqlx.SqlConn
}

func NewProductModel(conn sqlx.SqlConn) ProductModel {
	return &defaultProductModel{conn: conn}
}

func (m *defaultProductModel) Insert(ctx context.Context, data *Product) (*Product, error) {
	if data.ProductNo == "" {
		data.ProductNo = "P" + time.Now().Format("20060102150405") + fmt.Sprintf("%03d", time.Now().Nanosecond()/1e6)
	}
	if data.Status == "" {
		data.Status = ProductStatusDraft
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	data.CreateTime = now
	data.UpdateTime = now

	query := fmt.Sprintf(`INSERT INTO %s (product_no, name, category_id, description, main_image, status, price, market_price, stock, tags, freight_template_id, create_time, update_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, productTable)
	result, err := m.conn.ExecCtx(ctx, query, data.ProductNo, data.Name, data.CategoryId, data.Description, data.MainImage, data.Status, data.Price, data.MarketPrice, data.Stock, data.Tags, data.FreightTemplateId, data.CreateTime, data.UpdateTime)
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

func (m *defaultProductModel) FindOne(ctx context.Context, id int64) (*Product, error) {
	var p Product
	query := fmt.Sprintf(`SELECT id, product_no, name, category_id, description, main_image, status, price, market_price, stock, tags, freight_template_id, create_time, update_time FROM %s WHERE id = ?`, productTable)
	err := m.conn.QueryRowCtx(ctx, &p, query, id)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (m *defaultProductModel) FindList(ctx context.Context, categoryId int64, keyword, status string, page, size int) ([]*Product, int64, error) {
	where, args := buildProductWhere(categoryId, keyword, status)

	countQuery := fmt.Sprintf(`SELECT COUNT(1) FROM %s WHERE %s`, productTable, where)
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*Product{}, 0, nil
	}

	offset := (page - 1) * size
	listQuery := fmt.Sprintf(`SELECT id, product_no, name, category_id, description, main_image, status, price, market_price, stock, tags, freight_template_id, create_time, update_time FROM %s WHERE %s ORDER BY create_time DESC LIMIT ?, ?`, productTable, where)
	listArgs := append(args, offset, size)
	var list []*Product
	if err := m.conn.QueryRowsCtx(ctx, &list, listQuery, listArgs...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *defaultProductModel) Update(ctx context.Context, data *Product) error {
	query := fmt.Sprintf(`UPDATE %s SET name=?, category_id=?, description=?, main_image=?, price=?, market_price=?, stock=?, tags=?, freight_template_id=?, update_time=? WHERE id=?`, productTable)
	_, err := m.conn.ExecCtx(ctx, query, data.Name, data.CategoryId, data.Description, data.MainImage, data.Price, data.MarketPrice, data.Stock, data.Tags, data.FreightTemplateId, time.Now().Format("2006-01-02 15:04:05"), data.Id)
	return err
}

func (m *defaultProductModel) UpdateStatus(ctx context.Context, id int64, status string) error {
	query := fmt.Sprintf(`UPDATE %s SET status=?, update_time=? WHERE id=?`, productTable)
	_, err := m.conn.ExecCtx(ctx, query, status, time.Now().Format("2006-01-02 15:04:05"), id)
	return err
}

func (m *defaultProductModel) Delete(ctx context.Context, id int64) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, productTable)
	_, err := m.conn.ExecCtx(ctx, query, id)
	return err
}

func buildProductWhere(categoryId int64, keyword, status string) (string, []interface{}) {
	where := "1=1"
	var args []interface{}
	if categoryId > 0 {
		where += " AND category_id = ?"
		args = append(args, categoryId)
	}
	if keyword != "" {
		where += " AND name LIKE ?"
		args = append(args, "%"+keyword+"%")
	}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	return where, args
}
