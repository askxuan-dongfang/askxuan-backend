package model

import (
	"context"
	"errors"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type MasterProfileExt struct {
	MasterCode string `db:"master_code" json:"masterCode"`
	Bio        string `db:"bio" json:"bio"`
	Pricing    string `db:"pricing" json:"pricing"`
}

type MasterProfileExtModel interface {
	Find(ctx context.Context, masterCode string) (MasterProfileExt, error)
	Upsert(ctx context.Context, ext MasterProfileExt) error
}

type masterProfileExtModel struct{ conn sqlx.SqlConn }

func NewMasterProfileExtModel(conn sqlx.SqlConn) MasterProfileExtModel {
	return &masterProfileExtModel{conn: conn}
}

func (m *masterProfileExtModel) Find(ctx context.Context, masterCode string) (MasterProfileExt, error) {
	var ext MasterProfileExt
	err := m.conn.QueryRowCtx(ctx, &ext, "SELECT master_code,bio,pricing FROM master_profile_ext WHERE master_code=?", masterCode)
	if errors.Is(err, sqlx.ErrNotFound) {
		return MasterProfileExt{MasterCode: masterCode}, nil
	}
	return ext, err
}

func (m *masterProfileExtModel) Upsert(ctx context.Context, ext MasterProfileExt) error {
	_, err := m.conn.ExecCtx(ctx, `INSERT INTO master_profile_ext(master_code,bio,pricing) VALUES(?,?,?)
		ON DUPLICATE KEY UPDATE bio=VALUES(bio),pricing=VALUES(pricing)`, ext.MasterCode, ext.Bio, ext.Pricing)
	return err
}
