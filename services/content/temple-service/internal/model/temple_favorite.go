package model

import (
	"context"
	"fmt"

	"github.com/askxuan/temple-service/internal/types"
)

// templeFavoriteTable 寺院收藏表（位于 askxuan_temple 库）
const templeFavoriteTable = "temple_favorite"

// SetTempleFavorite 收藏/取消收藏寺院，返回操作后的收藏状态
func (m *defaultTempleModel) SetTempleFavorite(ctx context.Context, userId int64, templeCode string, favorited bool) (bool, error) {
	if favorited {
		_, err := m.conn.ExecCtx(ctx, "INSERT IGNORE INTO "+templeFavoriteTable+"(user_id,temple_code) VALUES(?,?)", userId, templeCode)
		return true, err
	}
	_, err := m.conn.ExecCtx(ctx, "DELETE FROM "+templeFavoriteTable+" WHERE user_id=? AND temple_code=?", userId, templeCode)
	return false, err
}

// ListFavoriteTemples 查询用户收藏的寺院列表（收藏时间倒序，上限 50）
func (m *defaultTempleModel) ListFavoriteTemples(ctx context.Context, userId int64) ([]types.Temple, error) {
	var rows []Temple
	query := fmt.Sprintf(`SELECT t.id, t.code, t.name, t.region, t.type, t.belief_code, t.sect, t.status, t.address, t.cover_image, t.rating, t.description, t.create_time, t.update_time FROM %s t JOIN %s f ON f.temple_code = t.code WHERE f.user_id = ? ORDER BY f.create_time DESC LIMIT 50`, templeTable, templeFavoriteTable)
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, userId); err != nil {
		return nil, err
	}
	list := make([]types.Temple, 0, len(rows))
	for i := range rows {
		list = append(list, types.Temple{
			Id:           rows[i].Code,
			Name:         rows[i].Name,
			Region:       rows[i].Region,
			Type:         rows[i].Type,
			BeliefCode:   rows[i].BeliefCode,
			Sect:         rows[i].Sect,
			Status:       rows[i].Status,
			Address:      rows[i].Address,
			CoverImage:   rows[i].CoverImage,
			Rating:       rows[i].Rating,
			Description:  rows[i].Description,
			ServiceCodes: []string{},
			ServiceTags:  []string{},
		})
	}
	return list, nil
}
