package model

import (
	"context"
	"fmt"
	"strings"

	"github.com/askxuan/master-service/internal/types"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// MasterServiceTag 大师服务标签（大师所提供，复用 S001-S013 目录）
type MasterServiceTag struct {
	Id          int64   `db:"id"`
	MasterCode  string  `db:"master_code"`
	ServiceCode string  `db:"service_code"`
	Price       float64 `db:"price"`
	Status      string  `db:"status"`
	CreateTime  string  `db:"create_time"`
	UpdateTime  string  `db:"update_time"`
}

const masterServiceTagTable = "master_service_tag"

// ListByMaster 查询某大师的标签列表
func (m *masterModel) ListServiceTagsByMaster(ctx context.Context, masterCode string) ([]*MasterServiceTag, error) {
	var list []*MasterServiceTag
	query := fmt.Sprintf("SELECT id, master_code, service_code, price, status, create_time, update_time FROM %s WHERE master_code = ? ORDER BY id", masterServiceTagTable)
	if err := m.conn.QueryRowsCtx(ctx, &list, query, masterCode); err != nil {
		return nil, err
	}
	return list, nil
}

// ReplaceServiceTags 覆盖式更新大师服务标签（事务）
func (m *masterModel) ReplaceServiceTags(ctx context.Context, masterCode string, tags []types.MasterServiceTagItem) error {
	return m.conn.TransactCtx(ctx, func(_ context.Context, s sqlx.Session) error {
		if _, err := s.ExecCtx(ctx, fmt.Sprintf("DELETE FROM %s WHERE master_code = ?", masterServiceTagTable), masterCode); err != nil {
			return err
		}
		for _, t := range tags {
			if _, err := s.ExecCtx(ctx,
				fmt.Sprintf("INSERT INTO %s (master_code, service_code, price, status) VALUES (?, ?, ?, 'enabled')", masterServiceTagTable),
				masterCode, t.ServiceCode, t.Price); err != nil {
				return err
			}
		}
		return nil
	})
}

// fixedServiceCatalog 固定服务目录（S001-S013，与统一数据字典一致）
var fixedServiceCatalog = map[string]bool{
	"S001": true, "S002": true, "S003": true, "S004": true, "S005": true,
	"S006": true, "S007": true, "S008": true, "S009": true, "S010": true,
	"S011": true, "S012": true, "S013": true,
}

// CountServiceType 固定服务目录校验（代码内置目录，避免跨库授权依赖）
func (m *masterModel) CountServiceType(_ context.Context, serviceCode string) (int64, error) {
	if fixedServiceCatalog[serviceCode] {
		return 1, nil
	}
	return 0, nil
}

// ListServiceTagsByMasters 批量查询多位法师的服务标签（按 master_code 分组，空入参返回空 map）
func (m *masterModel) ListServiceTagsByMasters(ctx context.Context, masterCodes []string) (map[string][]*MasterServiceTag, error) {
	result := make(map[string][]*MasterServiceTag, len(masterCodes))
	if len(masterCodes) == 0 {
		return result, nil
	}
	placeholders := make([]string, 0, len(masterCodes))
	args := make([]interface{}, 0, len(masterCodes))
	for _, code := range masterCodes {
		placeholders = append(placeholders, "?")
		args = append(args, code)
	}
	query := fmt.Sprintf(
		"SELECT id, master_code, service_code, price, status, create_time, update_time FROM %s WHERE master_code IN (%s) ORDER BY id",
		masterServiceTagTable, strings.Join(placeholders, ","))
	var list []*MasterServiceTag
	if err := m.conn.QueryRowsCtx(ctx, &list, query, args...); err != nil {
		return nil, err
	}
	for _, t := range list {
		result[t.MasterCode] = append(result[t.MasterCode], t)
	}
	return result, nil
}
