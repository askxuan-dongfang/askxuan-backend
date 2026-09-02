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
	Id              int64  `db:"id"`
	Code            string `db:"code"`
	Category        string `db:"category"`
	Name            string `db:"name"`
	Version         string `db:"version"`
	Description     string `db:"description"`
	Icon            string `db:"icon"`
	SourceType      string `db:"source_type"`
	SourceRef       string `db:"source_ref"`
	PromptTemplate  string `db:"prompt_template"`
	InputSchema     string `db:"input_schema"`
	RoutingKeywords string `db:"routing_keywords"`
	Capabilities    string `db:"capabilities"`
	ToolConfig      string `db:"tool_config"`
	RiskLevel       string `db:"risk_level"`
	SortOrder       int    `db:"sort_order"`
	Status          string `db:"status"`
	CreatedAt       string `db:"create_time"`
}

type SkillModel interface {
	List(ctx context.Context, status string) ([]*AISkill, error)
	FindByCode(ctx context.Context, code string) (*AISkill, error)
}
type skillModel struct{ conn sqlx.SqlConn }

func NewSkillModel(conn sqlx.SqlConn) SkillModel { return &skillModel{conn: conn} }

const skillRows = "id,code,category,name,version,description,icon,source_type,source_ref,prompt_template," +
	"COALESCE(CAST(input_schema AS CHAR),'{\"fields\":[]}') input_schema," +
	"COALESCE(CAST(routing_keywords AS CHAR),'[]') routing_keywords," +
	"COALESCE(CAST(capabilities AS CHAR),'[]') capabilities," +
	"COALESCE(CAST(tool_config AS CHAR),'{\"enabled\":false}') tool_config,risk_level,sort_order,status,create_time"

func (m *skillModel) List(ctx context.Context, status string) ([]*AISkill, error) {
	query := "SELECT " + skillRows + " FROM ai_skill"
	args := []interface{}{}
	if status != "" {
		query += " WHERE status=?"
		args = append(args, status)
	}
	query += " ORDER BY sort_order,id"
	var list []*AISkill
	err := m.conn.QueryRowsCtx(ctx, &list, query, args...)
	return list, err
}
func (m *skillModel) FindByCode(ctx context.Context, code string) (*AISkill, error) {
	var skill AISkill
	err := m.conn.QueryRowCtx(ctx, &skill, "SELECT "+skillRows+" FROM ai_skill WHERE code=?", code)
	return &skill, err
}
