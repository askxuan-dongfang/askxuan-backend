package model

import (
	"context"
	"fmt"

	"github.com/askxuan/product-service/internal/types"
)

// productFavoriteTable 商品收藏表（位于 askxuan_product 库）
const productFavoriteTable = "product_favorite"

// SetProductFavorite 收藏/取消收藏商品，返回操作后的收藏状态
func (m *defaultProductModel) SetProductFavorite(ctx context.Context, userId int64, productId int64, favorited bool) (bool, error) {
	if favorited {
		_, err := m.conn.ExecCtx(ctx, "INSERT IGNORE INTO "+productFavoriteTable+"(user_id,product_id) VALUES(?,?)", userId, productId)
		return true, err
	}
	_, err := m.conn.ExecCtx(ctx, "DELETE FROM "+productFavoriteTable+" WHERE user_id=? AND product_id=?", userId, productId)
	return false, err
}

// ListFavoriteProducts 查询用户收藏的商品列表（收藏时间倒序，上限 50）
func (m *defaultProductModel) ListFavoriteProducts(ctx context.Context, userId int64) ([]types.Product, error) {
	var rows []Product
	query := fmt.Sprintf(`SELECT p.id, p.product_no, p.name, p.category_id, p.description, p.main_image, p.status, p.price, p.market_price, p.stock, p.tags, p.freight_template_id, p.create_time, p.update_time FROM %s p JOIN %s f ON f.product_id = p.id WHERE f.user_id = ? ORDER BY f.create_time DESC LIMIT 50`, productTable, productFavoriteTable)
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, userId); err != nil {
		return nil, err
	}
	list := make([]types.Product, 0, len(rows))
	for i := range rows {
		list = append(list, types.Product{
			Id:                rows[i].Id,
			ProductNo:         rows[i].ProductNo,
			Name:              rows[i].Name,
			CategoryId:        rows[i].CategoryId,
			Description:       rows[i].Description,
			MainImage:         rows[i].MainImage,
			Status:            rows[i].Status,
			Price:             rows[i].Price,
			MarketPrice:       rows[i].MarketPrice,
			Stock:             rows[i].Stock,
			Tags:              rows[i].Tags,
			FreightTemplateId: rows[i].FreightTemplateId,
			CreateTime:        rows[i].CreateTime,
			UpdateTime:        rows[i].UpdateTime,
		})
	}
	return list, nil
}
