package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// addressTable 用户地址表
const addressTable = "user_address"

// UserAddress 收货地址实体（依据 init.sql user_address 表）
type UserAddress struct {
	Id         int64  `db:"id" json:"id"`
	UserId     int64  `db:"user_id" json:"userId"`
	Name       string `db:"name" json:"name"`
	Phone      string `db:"phone" json:"phone"`
	Province   string `db:"province" json:"province"`
	City       string `db:"city" json:"city"`
	District   string `db:"district" json:"district"`
	Detail     string `db:"detail" json:"detail"`
	IsDefault  int    `db:"is_default" json:"isDefault"` // 0否 1是
	CreateTime string `db:"create_time" json:"createTime"`
	UpdateTime string `db:"update_time" json:"updateTime"`
}

// AddressModel 用户地址模型接口
type AddressModel interface {
	ListByUser(ctx context.Context, userId int64) ([]*UserAddress, error)
	FindByID(ctx context.Context, id int64) (*UserAddress, error)
	Insert(ctx context.Context, data *UserAddress) (int64, error)
	Update(ctx context.Context, data *UserAddress) error
	Delete(ctx context.Context, id int64) error
	ClearDefaultByUser(ctx context.Context, userId int64) error
}

type defaultAddressModel struct {
	conn sqlx.SqlConn
}

// NewAddressModel 构造地址模型
func NewAddressModel(conn sqlx.SqlConn) AddressModel {
	return &defaultAddressModel{conn: conn}
}

// ListByUser 查询用户地址列表（默认地址置顶）
func (m *defaultAddressModel) ListByUser(ctx context.Context, userId int64) ([]*UserAddress, error) {
	query := fmt.Sprintf(`SELECT id, user_id, name, phone, province, city, district, detail, is_default, create_time, update_time FROM %s WHERE user_id = ? ORDER BY is_default DESC, id DESC`, addressTable)
	var list []*UserAddress
	err := m.conn.QueryRowsCtx(ctx, &list, query, userId)
	if err != nil {
		return nil, err
	}
	return list, nil
}

// FindByID 按 ID 查询地址
func (m *defaultAddressModel) FindByID(ctx context.Context, id int64) (*UserAddress, error) {
	var a UserAddress
	query := fmt.Sprintf(`SELECT id, user_id, name, phone, province, city, district, detail, is_default, create_time, update_time FROM %s WHERE id = ?`, addressTable)
	err := m.conn.QueryRowCtx(ctx, &a, query, id)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// Insert 创建地址，若 IsDefault=1 则先取消旧默认
func (m *defaultAddressModel) Insert(ctx context.Context, data *UserAddress) (int64, error) {
	if data.IsDefault == 1 {
		if err := m.ClearDefaultByUser(ctx, data.UserId); err != nil {
			return 0, err
		}
	}
	query := fmt.Sprintf(`INSERT INTO %s (user_id, name, phone, province, city, district, detail, is_default) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, addressTable)
	res, err := m.conn.ExecCtx(ctx, query,
		data.UserId, data.Name, data.Phone, data.Province,
		data.City, data.District, data.Detail, data.IsDefault)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// Update 更新地址
func (m *defaultAddressModel) Update(ctx context.Context, data *UserAddress) error {
	if data.IsDefault == 1 {
		if err := m.ClearDefaultByUser(ctx, data.UserId); err != nil {
			return err
		}
	}
	query := fmt.Sprintf(`UPDATE %s SET name = ?, phone = ?, province = ?, city = ?, district = ?, detail = ?, is_default = ? WHERE id = ?`, addressTable)
	_, err := m.conn.ExecCtx(ctx, query,
		data.Name, data.Phone, data.Province, data.City,
		data.District, data.Detail, data.IsDefault, data.Id)
	return err
}

// Delete 删除地址
func (m *defaultAddressModel) Delete(ctx context.Context, id int64) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, addressTable)
	_, err := m.conn.ExecCtx(ctx, query, id)
	return err
}

// ClearDefaultByUser 清除用户下所有默认地址标记
func (m *defaultAddressModel) ClearDefaultByUser(ctx context.Context, userId int64) error {
	query := fmt.Sprintf(`UPDATE %s SET is_default = 0 WHERE user_id = ? AND is_default = 1`, addressTable)
	_, err := m.conn.ExecCtx(ctx, query, userId)
	return err
}
