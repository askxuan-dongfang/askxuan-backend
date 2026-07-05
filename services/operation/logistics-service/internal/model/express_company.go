package model

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// 快递公司状态
const (
	ExpressStatusEnabled  = "enabled"
	ExpressStatusDisabled = "disabled"
)

const expressCompanyTable = "express_company"

// ExpressCompany 快递公司结构体
type ExpressCompany struct {
	Id              int64  `db:"id" json:"id"`
	Code            string `db:"code" json:"code"`
	Name            string `db:"name" json:"name"`
	LogoUrl         string `db:"logo_url" json:"logoUrl"`
	CustomerService string `db:"customer_service" json:"customerService"`
	Sort            int    `db:"sort" json:"sort"`
	Status          string `db:"status" json:"status"`
	CreateTime      string `db:"create_time" json:"createTime"`
	UpdateTime      string `db:"update_time" json:"updateTime"`
}

// ExpressCompanyModel 快递公司模型接口
type ExpressCompanyModel interface {
	Insert(ctx context.Context, data *ExpressCompany) (*ExpressCompany, error)
	FindList(ctx context.Context, code, name, status string, page, size int) ([]*ExpressCompany, int64, error)
	Update(ctx context.Context, data *ExpressCompany) error
}

type defaultExpressCompanyModel struct {
	conn sqlx.SqlConn
}

func NewExpressCompanyModel(conn sqlx.SqlConn) ExpressCompanyModel {
	return &defaultExpressCompanyModel{conn: conn}
}

func (m *defaultExpressCompanyModel) Insert(ctx context.Context, data *ExpressCompany) (*ExpressCompany, error) {
	if data.Status == "" {
		data.Status = ExpressStatusEnabled
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	data.CreateTime = now
	data.UpdateTime = now

	query := fmt.Sprintf(`INSERT INTO %s (code, name, logo_url, customer_service, sort, status, create_time, update_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, expressCompanyTable)
	result, err := m.conn.ExecCtx(ctx, query, data.Code, data.Name, data.LogoUrl, data.CustomerService, data.Sort, data.Status, data.CreateTime, data.UpdateTime)
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

func (m *defaultExpressCompanyModel) FindList(ctx context.Context, code, name, status string, page, size int) ([]*ExpressCompany, int64, error) {
	where, args := buildExpressWhere(code, name, status)

	countQuery := fmt.Sprintf(`SELECT COUNT(1) FROM %s WHERE %s`, expressCompanyTable, where)
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*ExpressCompany{}, 0, nil
	}

	offset := (page - 1) * size
	listQuery := fmt.Sprintf(`SELECT id, code, name, logo_url, customer_service, sort, status, create_time, update_time FROM %s WHERE %s ORDER BY sort ASC LIMIT ?, ?`, expressCompanyTable, where)
	listArgs := append(args, offset, size)
	var list []*ExpressCompany
	if err := m.conn.QueryRowsCtx(ctx, &list, listQuery, listArgs...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *defaultExpressCompanyModel) Update(ctx context.Context, data *ExpressCompany) error {
	query := fmt.Sprintf(`UPDATE %s SET name=?, logo_url=?, customer_service=?, sort=?, status=?, update_time=? WHERE id=?`, expressCompanyTable)
	_, err := m.conn.ExecCtx(ctx, query, data.Name, data.LogoUrl, data.CustomerService, data.Sort, data.Status, time.Now().Format("2006-01-02 15:04:05"), data.Id)
	return err
}

func buildExpressWhere(code, name, status string) (string, []interface{}) {
	where := "1=1"
	var args []interface{}
	if code != "" {
		where += " AND code = ?"
		args = append(args, code)
	}
	if name != "" {
		where += " AND name LIKE ?"
		args = append(args, "%"+name+"%")
	}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	return where, args
}
