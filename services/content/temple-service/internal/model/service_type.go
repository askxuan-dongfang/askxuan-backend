package model

import (
	"context"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ServiceType is the platform-owned catalog item a temple may enable.
type ServiceType struct {
	Id         int64  `db:"id" json:"id"`
	Code       string `db:"code" json:"code"`
	Name       string `db:"name" json:"name"`
	Category   string `db:"type" json:"category"`
	PriceRange string `db:"price_range" json:"priceRange"`
}

type ServiceTypeModel interface {
	FindAll(ctx context.Context) ([]*ServiceType, error)
	FindOne(ctx context.Context, code string) (*ServiceType, error)
}

type defaultServiceTypeModel struct {
	conn sqlx.SqlConn
}

func NewServiceTypeModel(conn sqlx.SqlConn) ServiceTypeModel {
	return &defaultServiceTypeModel{conn: conn}
}

func (m *defaultServiceTypeModel) FindAll(ctx context.Context) ([]*ServiceType, error) {
	var rows []*ServiceType
	err := m.conn.QueryRowsCtx(ctx, &rows,
		`SELECT id,code,name,type,price_range FROM service_type ORDER BY code`)
	if rows == nil {
		rows = []*ServiceType{}
	}
	return rows, err
}

func (m *defaultServiceTypeModel) FindOne(ctx context.Context, code string) (*ServiceType, error) {
	var row ServiceType
	err := m.conn.QueryRowCtx(ctx, &row,
		`SELECT id,code,name,type,price_range FROM service_type WHERE code=?`, strings.TrimSpace(code))
	return &row, err
}
