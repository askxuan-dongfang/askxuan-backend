package model

import (
	"context"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type MasterImage struct {
	URL     string `db:"url" json:"url"`
	Caption string `db:"caption" json:"caption"`
	Kind    string `db:"kind" json:"kind"`
}
type MasterImageModel interface {
	FindPublic(context.Context, string) ([]MasterImage, error)
}
type masterImageModel struct{ conn sqlx.SqlConn }

func NewMasterImageModel(conn sqlx.SqlConn) MasterImageModel { return &masterImageModel{conn} }
func (m *masterImageModel) FindPublic(ctx context.Context, code string) ([]MasterImage, error) {
	images := []MasterImage{}
	// Demo galleries only belong to the original name and avatar. An operator's
	// new identity or upload must never inherit another person's demonstration.
	err := m.conn.QueryRowsCtx(ctx, &images, `SELECT i.url,i.caption,i.kind FROM master_image i JOIN master m ON m.code=i.master_code WHERE m.code=? AND m.dharma_name=i.name_guard AND m.avatar=i.avatar_guard AND m.auth_status='已认证' AND m.shelf_status='on_shelf' AND m.platform_status='normal' ORDER BY i.sort,i.url`, code)
	return images, err
}
