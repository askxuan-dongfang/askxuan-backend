package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	SkillStatusEnabled = "enabled"
	SkillCodeGeneral   = "general"
)

type AISkill struct {
	Id             int64  `db:"id"`
	Code           string `db:"code"`
	Name           string `db:"name"`
	Description    string `db:"description"`
	Icon           string `db:"icon"`
	PromptTemplate string `db:"prompt_template"`
	Status         string `db:"status"`
	CreatedAt      string `db:"create_time"`
}

type SkillModel interface {
	List(ctx context.Context, status string) ([]*AISkill, error)
	FindByCode(ctx context.Context, code string) (*AISkill, error)
}
type skillModel struct{ conn sqlx.SqlConn }

func NewSkillModel(conn sqlx.SqlConn) SkillModel { return &skillModel{conn: conn} }
func (m *skillModel) List(ctx context.Context, status string) ([]*AISkill, error) {
	query := "SELECT id,code,name,description,icon,prompt_template,status,create_time FROM ai_skill"
	args := []interface{}{}
	if status != "" {
		query += " WHERE status=?"
		args = append(args, status)
	}
	query += " ORDER BY id"
	var list []*AISkill
	err := m.conn.QueryRowsCtx(ctx, &list, query, args...)
	return list, err
}
func (m *skillModel) FindByCode(ctx context.Context, code string) (*AISkill, error) {
	var skill AISkill
	err := m.conn.QueryRowCtx(ctx, &skill, "SELECT id,code,name,description,icon,prompt_template,status,create_time FROM ai_skill WHERE code=?", code)
	return &skill, err
}
