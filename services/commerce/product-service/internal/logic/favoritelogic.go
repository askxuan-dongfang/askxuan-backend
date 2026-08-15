package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/product-service/internal/svc"
	"github.com/askxuan/product-service/internal/types"
)

// SetProductFavorite 收藏/取消收藏商品
func SetProductFavorite(ctx context.Context, s *svc.ServiceContext, userId int64, productId int64, favorited bool) (*types.FavoriteResp, error) {
	if productId <= 0 {
		return nil, common.ErrParam
	}
	value, err := s.ProductModel.SetProductFavorite(ctx, userId, productId, favorited)
	if err != nil {
		return nil, common.ErrSystem
	}
	return &types.FavoriteResp{Favorited: value}, nil
}

// ListFavoriteProducts 查询用户收藏的商品列表（收藏时间倒序，上限 50）
func ListFavoriteProducts(ctx context.Context, s *svc.ServiceContext, userId int64) (*types.FavoritesResp, error) {
	list, err := s.ProductModel.ListFavoriteProducts(ctx, userId)
	if err != nil {
		return nil, common.ErrSystem
	}
	return &types.FavoritesResp{List: list}, nil
}
