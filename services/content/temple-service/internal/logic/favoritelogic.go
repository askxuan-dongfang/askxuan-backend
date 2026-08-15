package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/temple-service/internal/svc"
	"github.com/askxuan/temple-service/internal/types"
)

// SetTempleFavorite 收藏/取消收藏寺院（:id 为寺院编码）
func SetTempleFavorite(ctx context.Context, s *svc.ServiceContext, userId int64, templeCode string, favorited bool) (*types.FavoriteResp, error) {
	if templeCode == "" {
		return nil, common.ErrParam
	}
	value, err := s.TempleModel.SetTempleFavorite(ctx, userId, templeCode, favorited)
	if err != nil {
		return nil, common.ErrSystem
	}
	return &types.FavoriteResp{Favorited: value}, nil
}

// ListFavoriteTemples 查询用户收藏的寺院列表（收藏时间倒序，上限 50）
func ListFavoriteTemples(ctx context.Context, s *svc.ServiceContext, userId int64) (*types.FavoritesResp, error) {
	list, err := s.TempleModel.ListFavoriteTemples(ctx, userId)
	if err != nil {
		return nil, common.ErrSystem
	}
	return &types.FavoritesResp{List: list}, nil
}
