package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// templateTable 消息模板表（位于 askxuan_message 库）
const templateTable = "message_template"

// MessageTemplate 消息模板实体
type MessageTemplate struct {
	Id              int64  `db:"id" json:"id"`
	Code            string `db:"code" json:"code"`
	TitleTemplate   string `db:"title_template" json:"titleTemplate"`
	ContentTemplate string `db:"content_template" json:"contentTemplate"`
	Variables       string `db:"variables" json:"variables"`
	Type            string `db:"type" json:"type"`
	CreateTime      string `db:"create_time" json:"createTime"`
	UpdateTime      string `db:"update_time" json:"updateTime"`
}

// TemplateModel 消息模板模型接口
type TemplateModel interface {
	List(ctx context.Context, tType string, page, size int) ([]*MessageTemplate, int64, error)
	Insert(ctx context.Context, t *MessageTemplate) (int64, error)
	Update(ctx context.Context, t *MessageTemplate) error
}

type defaultTemplateModel struct {
	conn sqlx.SqlConn
}

// NewTemplateModel 构造消息模板模型
func NewTemplateModel(conn sqlx.SqlConn) TemplateModel {
	return &defaultTemplateModel{conn: conn}
}

// templateRows 模板表查询字段
const templateRows = "id, code, title_template, content_template, variables, type, create_time, update_time"

// List 查询模板列表，tType 非空时按类型筛选，按 id 倒序分页
func (m *defaultTemplateModel) List(ctx context.Context, tType string, page, size int) ([]*MessageTemplate, int64, error) {
	where := "WHERE 1=1"
	args := []interface{}{}
	if tType != "" {
		where += " AND type = ?"
		args = append(args, tType)
	}

	countQuery := fmt.Sprintf("SELECT COUNT(1) FROM %s %s", templateTable, where)
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*MessageTemplate{}, 0, nil
	}

	offset := (page - 1) * size
	listQuery := fmt.Sprintf("SELECT %s FROM %s %s ORDER BY id DESC LIMIT ?, ?", templateRows, templateTable, where)
	listArgs := append(args, offset, size)
	var list []*MessageTemplate
	if err := m.conn.QueryRowsCtx(ctx, &list, listQuery, listArgs...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// Insert 新建模板，返回自增 ID
func (m *defaultTemplateModel) Insert(ctx context.Context, data *MessageTemplate) (int64, error) {
	const query = `INSERT INTO ` + templateTable + ` (code, title_template, content_template, variables, type) VALUES (?, ?, ?, ?, ?)`
	res, err := m.conn.ExecCtx(ctx, query,
		data.Code, data.TitleTemplate, data.ContentTemplate, data.Variables, data.Type)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// Update 更新模板（title_template, content_template, variables, type）
func (m *defaultTemplateModel) Update(ctx context.Context, data *MessageTemplate) error {
	const query = `UPDATE ` + templateTable + ` SET title_template = ?, content_template = ?, variables = ?, type = ? WHERE id = ?`
	_, err := m.conn.ExecCtx(ctx, query,
		data.TitleTemplate, data.ContentTemplate, data.Variables, data.Type, data.Id)
	return err
}
