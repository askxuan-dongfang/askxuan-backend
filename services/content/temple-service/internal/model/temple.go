package model

import (
	"context"
	"fmt"
	"github.com/askxuan/common"
	"strings"

	"github.com/askxuan/temple-service/internal/types"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ============ 寺院状态常量 ============

const (
	TempleStatusNormal    = "正常"  // 正常运营
	TempleStatusPending   = "待审核" // 待审核
	TempleStatusBanned    = "封禁"  // 平台封禁
	TempleStatusRecommend = "推荐"  // 平台推荐
)

// ============ 寺院编码常量（与统一数据字典第1节一致） ============

const (
	T001 = "T001"
	T002 = "T002"
	T003 = "T003"
	T004 = "T004"
	T005 = "T005"
	T006 = "T006"
)

// templeTable 寺院表（位于 askxuan 默认库）
const templeTable = "temple"

// Temple 寺院实体（依据 init.sql temple 表）
type Temple struct {
	AuthStatus     string  `db:"auth_status" json:"authStatus"`
	PlatformStatus string  `db:"platform_status" json:"platformStatus"`
	ShelfStatus    string  `db:"shelf_status" json:"shelfStatus"`
	Recommended    bool    `db:"recommended" json:"recommended"`
	StateVersion   int64   `db:"state_version" json:"stateVersion"`
	TraditionCodes string  `db:"tradition_codes"`
	RegionCode     string  `db:"region_code" json:"regionCode"`
	RegionName     string  `db:"region_name" json:"regionName"`
	Id             int64   `db:"id" json:"id"`         // 自增主键
	Code           string  `db:"code" json:"code"`     // 寺院编码 T001~T006
	Name           string  `db:"name" json:"name"`     // 名称
	Region         string  `db:"region" json:"region"` // 地区
	Type           string  `db:"type" json:"type"`     // 类型 汉传佛教/道教/藏传佛教
	BeliefCode     string  `db:"belief_code" json:"beliefCode"`
	Sect           string  `db:"sect" json:"sect"`       // 宗派 禅宗/全真派/格鲁派/正一派
	Status         string  `db:"status" json:"status"`   // 状态 正常/待审核/封禁/推荐
	Address        string  `db:"address" json:"address"` // 地址
	CoverImage     string  `db:"cover_image" json:"coverImage"`
	Rating         float64 `db:"rating" json:"rating"` // 评分
	Description    string  `db:"description" json:"description"`
	CreateTime     string  `db:"create_time" json:"createTime"`
	UpdateTime     string  `db:"update_time" json:"updateTime"`
}

// TempleFilter 列表查询过滤条件
type TempleFilter struct {
	TraditionCode string
	ServiceCode   string
	RegionCode    string
	BeliefCode    string
	Sect          string
	Type          string
	Region        string
	Status        string // 为空时不筛选状态
	PublicOnly    bool
	Statuses      []string
}

func TemplePublicStatuses() []string {
	return []string{TempleStatusNormal, TempleStatusRecommend}
}

func IsTemplePublicStatus(status string) bool {
	return status == TempleStatusNormal || status == TempleStatusRecommend
}

// TempleModel 寺院模型接口
type TempleModel interface {
	Insert(ctx context.Context, data *Temple) (int64, error)
	FindOne(ctx context.Context, code string) (*Temple, error)
	FindOneByPk(ctx context.Context, id int64) (*Temple, error)
	FindList(ctx context.Context, filter TempleFilter, page, size int) ([]*Temple, int64, error)
	FindListByStatus(ctx context.Context, status string, page, size int) ([]*Temple, int64, error)
	Update(ctx context.Context, data *Temple) error
	UpdateStatus(ctx context.Context, id int64, status string) error
	Delete(ctx context.Context, id int64) error
	SetTempleFavorite(ctx context.Context, userId int64, templeCode string, favorited bool) (bool, error)
	ListFavoriteTemples(ctx context.Context, userId int64) ([]types.Temple, error)
}

type defaultTempleModel struct {
	conn sqlx.SqlConn
}

// NewTempleModel 构造寺院模型
func NewTempleModel(conn sqlx.SqlConn) TempleModel {
	return &defaultTempleModel{conn: conn}
}

// Insert 新建寺院，返回自增 ID
func (m *defaultTempleModel) Insert(ctx context.Context, data *Temple) (int64, error) {
	const query = `INSERT INTO ` + templeTable + ` (code, name, region, type, belief_code, sect, status, address, cover_image, rating, description,auth_status,platform_status,shelf_status,recommended) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,?,?,?,?)`
	res, err := m.conn.ExecCtx(ctx, query,
		data.Code, data.Name, data.Region, data.Type, data.BeliefCode, data.Sect,
		data.Status, data.Address, data.CoverImage, data.Rating, data.Description, initialTempleAuth(data.Status), initialTemplePlatform(data.Status), "on_shelf", data.Status == TempleStatusRecommend)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// FindOne 按寺院编码查询
func (m *defaultTempleModel) FindOne(ctx context.Context, code string) (*Temple, error) {
	var t Temple
	query := fmt.Sprintf(`SELECT id, code, name, region, type, belief_code, sect, status, auth_status, platform_status, shelf_status, recommended, state_version, address, cover_image, rating, description, create_time, update_time, `+templeRegionColumns("temple.code")+", "+common.DirectoryTagColumns("temple", "temple.code")+` FROM %s WHERE code = ?`, templeTable)
	err := m.conn.QueryRowCtx(ctx, &t, query, code)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// FindOneByPk 按自增主键查询（管理台从 JWT 取 templeId 后使用）
func (m *defaultTempleModel) FindOneByPk(ctx context.Context, id int64) (*Temple, error) {
	var t Temple
	query := fmt.Sprintf(`SELECT id, code, name, region, type, belief_code, sect, status, auth_status, platform_status, shelf_status, recommended, state_version, address, cover_image, rating, description, create_time, update_time, `+templeRegionColumns("temple.code")+", "+common.DirectoryTagColumns("temple", "temple.code")+` FROM %s WHERE id = ?`, templeTable)
	err := m.conn.QueryRowCtx(ctx, &t, query, id)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// FindList 列表查询，支持按 sect/type/region/status 筛选 + 分页
func (m *defaultTempleModel) FindList(ctx context.Context, filter TempleFilter, page, size int) ([]*Temple, int64, error) {
	where, args := buildTempleWhere(filter)

	countQuery := fmt.Sprintf(`SELECT COUNT(1) FROM %s WHERE %s`, templeTable, where)
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*Temple{}, 0, nil
	}

	offset := (page - 1) * size
	listQuery := fmt.Sprintf(`SELECT id, code, name, region, type, belief_code, sect, status, auth_status, platform_status, shelf_status, recommended, state_version, address, cover_image, rating, description, create_time, update_time, `+templeRegionColumns("temple.code")+", "+common.DirectoryTagColumns("temple", "temple.code")+` FROM %s WHERE %s ORDER BY id ASC LIMIT ?, ?`,
		templeTable, where)
	listArgs := append(args, offset, size)
	var list []*Temple
	if err := m.conn.QueryRowsCtx(ctx, &list, listQuery, listArgs...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// FindListByStatus 按状态筛选 + 分页（平台审核用）
func (m *defaultTempleModel) FindListByStatus(ctx context.Context, status string, page, size int) ([]*Temple, int64, error) {
	return m.FindList(ctx, TempleFilter{Status: status}, page, size)
}

// Update 更新寺院信息
func (m *defaultTempleModel) Update(ctx context.Context, data *Temple) error {
	const query = `UPDATE ` + templeTable + ` SET name = ?, region = ?, type = ?, belief_code = ?, sect = ?, address = ?, cover_image = ?, rating = ?, description = ? WHERE id = ?`
	return m.conn.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var previous string
		if err := tx.QueryRowCtx(ctx, &previous, "SELECT belief_code FROM temple WHERE id=? AND code=? FOR UPDATE", data.Id, data.Code); err != nil {
			return err
		}
		canonical, err := common.CanonicalBelief(data.BeliefCode)
		if err != nil && previous != data.BeliefCode {
			return err
		}
		old, _ := common.CanonicalBelief(previous)
		if err == nil {
			data.BeliefCode = canonical
		}
		if canonical != old {
			existing, err := common.ProviderTraditions(ctx, tx, "temple", data.Code)
			if err != nil {
				return err
			}
			if err := common.ValidateTraditions(ctx, tx, canonical, existing, existing, previous, false); err != nil {
				return err
			}
		}
		_, err = tx.ExecCtx(ctx, query,
			data.Name, data.Region, data.Type, data.BeliefCode, data.Sect,
			data.Address, data.CoverImage, data.Rating, data.Description, data.Id)
		return err
	})
}

// UpdateStatus 更新寺院状态
func (m *defaultTempleModel) UpdateStatus(ctx context.Context, id int64, status string) error {
	return m.updateLegacyState(ctx, id, status)
}

// Delete 删除寺院
func (m *defaultTempleModel) Delete(ctx context.Context, id int64) error {
	const query = `DELETE FROM ` + templeTable + ` WHERE id = ?`
	_, err := m.conn.ExecCtx(ctx, query, id)
	return err
}

// buildTempleWhere 构建寺院列表 WHERE 子句
func buildTempleWhere(filter TempleFilter) (string, []interface{}) {
	where := "1=1"
	if filter.PublicOnly {
		where += " AND auth_status='certified' AND platform_status='normal' AND shelf_status='on_shelf'"
	}
	var args []interface{}
	if filter.ServiceCode != "" {
		where += " AND EXISTS (SELECT 1 FROM temple_service ts WHERE ts.temple_code=temple.code AND ts.service_code=? AND ts.status='on_shelf')"
		args = append(args, filter.ServiceCode)
	}
	if filter.BeliefCode == "daoism" || filter.BeliefCode == "taoism" {
		where += " AND belief_code IN (?,?)"
		args = append(args, "daoism", "taoism")
	} else if filter.BeliefCode != "" {
		where += " AND belief_code = ?"
		args = append(args, filter.BeliefCode)
	}
	if filter.Sect != "" {
		where += " AND sect = ?"
		args = append(args, filter.Sect)
	}
	if filter.TraditionCode != "" {
		where += " AND EXISTS (SELECT 1 FROM askxuan_temple.provider_directory_tag pdt WHERE pdt.provider_kind='temple' AND pdt.provider_code=temple.code AND pdt.tag_code=?)"
		args = append(args, filter.TraditionCode)
	}
	if filter.Type != "" {
		where += " AND type = ?"
		args = append(args, filter.Type)
	}
	if filter.RegionCode != "" {
		where += " AND " + templeRegionCode("temple.code") + " = ?"
		args = append(args, filter.RegionCode)
	}
	if filter.Region != "" {
		where += " AND region = ?"
		args = append(args, filter.Region)
	}
	if filter.Status != "" {
		where += " AND status = ?"
		args = append(args, filter.Status)
	} else if len(filter.Statuses) > 0 {
		placeholders := make([]string, len(filter.Statuses))
		for i, status := range filter.Statuses {
			placeholders[i] = "?"
			args = append(args, status)
		}
		where += " AND status IN (" + strings.Join(placeholders, ",") + ")"
	}
	return where, args
}
