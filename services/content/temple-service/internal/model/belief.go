package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	BeliefHanBuddhism     = "han_buddhism"
	BeliefTibetanBuddhism = "tibetan_buddhism"
	BeliefDaoism          = "daoism"
	BeliefFolk            = "folk"
)

type BeliefProfile struct {
	Code        string `db:"code"`
	Name        string `db:"name"`
	Summary     string `db:"summary"`
	Description string `db:"description"`
	CoverImage  string `db:"cover_image"`
	Sort        int    `db:"sort"`
	Status      string `db:"status"`
}

type BeliefModel interface {
	FindOne(ctx context.Context, code string) (*BeliefProfile, error)
	Update(ctx context.Context, profile *BeliefProfile) error
}

type beliefModel struct{ conn sqlx.SqlConn }

func NewBeliefModel(conn sqlx.SqlConn) BeliefModel { return &beliefModel{conn: conn} }

func (m *beliefModel) FindOne(ctx context.Context, code string) (*BeliefProfile, error) {
	var profile BeliefProfile
	err := m.conn.QueryRowCtx(ctx, &profile, "SELECT code,name,summary,description,cover_image,sort,status FROM belief_profile WHERE code=? AND status='enabled'", code)
	return &profile, err
}

func (m *beliefModel) Update(ctx context.Context, profile *BeliefProfile) error {
	_, err := m.conn.ExecCtx(ctx, "UPDATE belief_profile SET name=?,summary=?,description=?,cover_image=?,sort=? WHERE code=?", profile.Name, profile.Summary, profile.Description, profile.CoverImage, profile.Sort, profile.Code)
	return err
}

func IsValidBeliefCode(code string) bool {
	switch code {
	case BeliefHanBuddhism, BeliefTibetanBuddhism, BeliefDaoism, BeliefFolk:
		return true
	default:
		return false
	}
}
