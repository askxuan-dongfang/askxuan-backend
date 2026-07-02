package model

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// 运费模板状态
const (
	TemplateStatusEnabled  = "enabled"
	TemplateStatusDisabled = "disabled"
)

// 计费方式
const (
	FreightTypeByWeight = "by_weight"
	FreightTypeByPiece  = "by_piece"
)

const freightTemplateTable = "askxuan_logistics.freight_template"

// FreightTemplate 运费模板结构体
type FreightTemplate struct {
	Id           int64  `db:"id" json:"id"`
	Name         string `db:"name" json:"name"`
	Type         string `db:"type" json:"type"`
	FreeShipping int    `db:"free_shipping" json:"freeShipping"`
	Config       string `db:"config" json:"config"`
	Status       string `db:"status" json:"status"`
	CreateTime   string `db:"create_time" json:"createTime"`
	UpdateTime   string `db:"update_time" json:"updateTime"`
}

// FreightTemplateModel 运费模板模型接口
type FreightTemplateModel interface {
	Insert(ctx context.Context, data *FreightTemplate) (*FreightTemplate, error)
	FindList(ctx context.Context, name, typ, status string, page, size int) ([]*FreightTemplate, int64, error)
	Update(ctx context.Context, data *FreightTemplate) error
}

type defaultFreightTemplateModel struct {
	conn sqlx.SqlConn
}

func NewFreightTemplateModel(conn sqlx.SqlConn) FreightTemplateModel {
	return &defaultFreightTemplateModel{conn: conn}
}

func (m *defaultFreightTemplateModel) Insert(ctx context.Context, data *FreightTemplate) (*FreightTemplate, error) {
	if data.Type == "" {
		data.Type = FreightTypeByWeight
	}
	if data.Status == "" {
		data.Status = TemplateStatusEnabled
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	data.CreateTime = now
	data.UpdateTime = now

	query := fmt.Sprintf(`INSERT INTO %s (name, type, free_shipping, config, status, create_time, update_time) VALUES (?, ?, ?, ?, ?, ?, ?)`, freightTemplateTable)
	result, err := m.conn.ExecCtx(ctx, query, data.Name, data.Type, data.FreeShipping, data.Config, data.Status, data.CreateTime, data.UpdateTime)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	data.Id = id
	return data, nil
}

func (m *defaultFreightTemplateModel) FindList(ctx context.Context, name, typ, status string, page, size int) ([]*FreightTemplate, int64, error) {
	where, args := buildFreightWhere(name, typ, status)

	countQuery := fmt.Sprintf(`SELECT COUNT(1) FROM %s WHERE %s`, freightTemplateTable, where)
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*FreightTemplate{}, 0, nil
	}

	offset := (page - 1) * size
	listQuery := fmt.Sprintf(`SELECT id, name, type, free_shipping, config, status, create_time, update_time FROM %s WHERE %s ORDER BY create_time DESC LIMIT ?, ?`, freightTemplateTable, where)
	listArgs := append(args, offset, size)
	var list []*FreightTemplate
	if err := m.conn.QueryRowsCtx(ctx, &list, listQuery, listArgs...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *defaultFreightTemplateModel) Update(ctx context.Context, data *FreightTemplate) error {
	query := fmt.Sprintf(`UPDATE %s SET name=?, type=?, free_shipping=?, config=?, status=?, update_time=? WHERE id=?`, freightTemplateTable)
	_, err := m.conn.ExecCtx(ctx, query, data.Name, data.Type, data.FreeShipping, data.Config, data.Status, time.Now().Format("2006-01-02 15:04:05"), data.Id)
	return err
}

func buildFreightWhere(name, typ, status string) (string, []interface{}) {
	where := "1=1"
	var args []interface{}
	if name != "" {
		where += " AND name LIKE ?"
		args = append(args, "%"+name+"%")
	}
	if typ != "" {
		where += " AND type = ?"
		args = append(args, typ)
	}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	return where, args
}
