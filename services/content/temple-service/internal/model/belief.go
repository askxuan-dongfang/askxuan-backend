package model

import (
	"context"
	"regexp"

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
	Icon        string `db:"icon"`
	Sort        int    `db:"sort"`
	Status      string `db:"status"`
}

type BeliefModel interface {
	FindOne(ctx context.Context, code string) (*BeliefProfile, error)
	FindAll(ctx context.Context, includeDisabled bool) ([]*BeliefProfile, error)
	Insert(ctx context.Context, profile *BeliefProfile) error
	Update(ctx context.Context, profile *BeliefProfile) error
	UpdateStatus(ctx context.Context, code, status string) error
}

type beliefModel struct{ conn sqlx.SqlConn }

func NewBeliefModel(conn sqlx.SqlConn) BeliefModel { return &beliefModel{conn: conn} }

func (m *beliefModel) FindOne(ctx context.Context, code string) (*BeliefProfile, error) {
	var profile BeliefProfile
	err := m.conn.QueryRowCtx(ctx, &profile, "SELECT code,name,summary,description,cover_image,icon,sort,status FROM belief_profile WHERE code=? AND status='enabled'", code)
	return &profile, err
}

func (m *beliefModel) FindAll(ctx context.Context, includeDisabled bool) ([]*BeliefProfile, error) {
	query := "SELECT code,name,summary,description,cover_image,icon,sort,status FROM belief_profile"
	if !includeDisabled {
		query += " WHERE status='enabled'"
	}
	query += " ORDER BY sort,code"
	var profiles []*BeliefProfile
	err := m.conn.QueryRowsCtx(ctx, &profiles, query)
	return profiles, err
}

func (m *beliefModel) Insert(ctx context.Context, profile *BeliefProfile) error {
	_, err := m.conn.ExecCtx(ctx, "INSERT INTO belief_profile(code,name,summary,description,cover_image,icon,sort,status) VALUES(?,?,?,?,?,?,?,?)", profile.Code, profile.Name, profile.Summary, profile.Description, profile.CoverImage, profile.Icon, profile.Sort, profile.Status)
	return err
}

func (m *beliefModel) Update(ctx context.Context, profile *BeliefProfile) error {
	_, err := m.conn.ExecCtx(ctx, "UPDATE belief_profile SET name=?,summary=?,description=?,cover_image=?,icon=?,sort=? WHERE code=?", profile.Name, profile.Summary, profile.Description, profile.CoverImage, profile.Icon, profile.Sort, profile.Code)
	return err
}

func (m *beliefModel) UpdateStatus(ctx context.Context, code, status string) error {
	_, err := m.conn.ExecCtx(ctx, "UPDATE belief_profile SET status=? WHERE code=?", status, code)
	return err
}

func IsValidBeliefCode(code string) bool {
	return regexp.MustCompile(`^[a-z][a-z0-9_]{2,31}$`).MatchString(code)
}
