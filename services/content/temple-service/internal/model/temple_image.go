package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ============ 寺院图片 MySQL 存储 ============

// 图片类型常量
const (
	ImageTypeCover  = "cover"  // 封面图（单张）
	ImageTypeDetail = "detail" // 详情图（最多9张）
	ImageTypeHero   = "hero"   // Hero图（单张）
)

// templeImageTable 寺院图片表（位于 askxuan_temple 库）
const templeImageTable = "askxuan_temple.temple_image"

// TempleImage 寺院图片
type TempleImage struct {
	Id         int64  `db:"id" json:"id"`
	TempleCode string `db:"temple_code" json:"templeCode"`
	Url        string `db:"url" json:"url"`
	Type       string `db:"type" json:"type"` // cover/detail/hero
	Sort       int    `db:"sort" json:"sort"`
	CreateTime string `db:"create_time" json:"createTime"`
}

// TempleImageModel 寺院图片模型接口
type TempleImageModel interface {
	Insert(ctx context.Context, data *TempleImage) (int64, error)
	FindOne(ctx context.Context, id int64) (*TempleImage, error)
	FindByTempleCode(ctx context.Context, templeCode string) ([]*TempleImage, error)
	CountByTempleCodeAndType(ctx context.Context, templeCode, imgType string) (int64, error)
	Update(ctx context.Context, data *TempleImage) error
	Delete(ctx context.Context, id int64) error
}

type defaultTempleImageModel struct {
	conn sqlx.SqlConn
}

// NewTempleImageModel 构造寺院图片模型
func NewTempleImageModel(conn sqlx.SqlConn) TempleImageModel {
	return &defaultTempleImageModel{conn: conn}
}

// Insert 新增寺院图片，返回自增 ID
func (m *defaultTempleImageModel) Insert(ctx context.Context, data *TempleImage) (int64, error) {
	query := fmt.Sprintf(`INSERT INTO %s (temple_code, url, type, sort) VALUES (?, ?, ?, ?)`, templeImageTable)
	res, err := m.conn.ExecCtx(ctx, query, data.TempleCode, data.Url, data.Type, data.Sort)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// FindOne 按 ID 查询寺院图片
func (m *defaultTempleImageModel) FindOne(ctx context.Context, id int64) (*TempleImage, error) {
	var img TempleImage
	query := fmt.Sprintf(`SELECT id, temple_code, url, type, sort, create_time FROM %s WHERE id = ?`, templeImageTable)
	err := m.conn.QueryRowCtx(ctx, &img, query, id)
	if err != nil {
		return nil, err
	}
	return &img, nil
}

// FindByTempleCode 查询寺院图片列表
func (m *defaultTempleImageModel) FindByTempleCode(ctx context.Context, templeCode string) ([]*TempleImage, error) {
	query := fmt.Sprintf(`SELECT id, temple_code, url, type, sort, create_time FROM %s WHERE temple_code = ? ORDER BY sort ASC, id ASC`, templeImageTable)
	var list []*TempleImage
	if err := m.conn.QueryRowsCtx(ctx, &list, query, templeCode); err != nil {
		return nil, err
	}
	return list, nil
}

// CountByTempleCodeAndType 统计寺院某类型图片数量（用于配额校验）
func (m *defaultTempleImageModel) CountByTempleCodeAndType(ctx context.Context, templeCode, imgType string) (int64, error) {
	query := fmt.Sprintf(`SELECT COUNT(1) FROM %s WHERE temple_code = ? AND type = ?`, templeImageTable)
	var count int64
	if err := m.conn.QueryRowCtx(ctx, &count, query, templeCode, imgType); err != nil {
		return 0, err
	}
	return count, nil
}

// Update 更新寺院图片
func (m *defaultTempleImageModel) Update(ctx context.Context, data *TempleImage) error {
	query := fmt.Sprintf(`UPDATE %s SET temple_code = ?, url = ?, type = ?, sort = ? WHERE id = ?`, templeImageTable)
	_, err := m.conn.ExecCtx(ctx, query, data.TempleCode, data.Url, data.Type, data.Sort, data.Id)
	return err
}

// Delete 删除寺院图片
func (m *defaultTempleImageModel) Delete(ctx context.Context, id int64) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, templeImageTable)
	_, err := m.conn.ExecCtx(ctx, query, id)
	return err
}
