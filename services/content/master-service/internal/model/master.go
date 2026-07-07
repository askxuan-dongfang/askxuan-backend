package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ============ 法师状态常量 ============

// 法师上下架状态常量
const (
	MasterShelfStatusOnShelf  = "on_shelf"  // 上架（C端可见）
	MasterShelfStatusOffShelf = "off_shelf" // 下架（C端不可见）
)

// 法师平台状态常量
const (
	MasterPlatformStatusNormal = "normal" // 正常
	MasterPlatformStatusBanned = "banned" // 封禁
)

// 法师认证状态常量
const (
	MasterAuthStatusVerified = "已认证"
	MasterAuthStatusPending  = "待审核"
)

// 寺院编码常量（与 temple-service 对齐，用于关联）
const (
	T001 = "T001"
	T002 = "T002"
	T003 = "T003"
	T004 = "T004"
	T005 = "T005"
	T006 = "T006"
)

// Master 法师表（对应 askxuan.master）
type Master struct {
	Id             int64   `db:"id" json:"id"`
	Code           string  `db:"code" json:"code"` // 法师编码 M001~M006
	DharmaName     string  `db:"dharma_name" json:"dharmaName"`
	LayName        string  `db:"lay_name" json:"layName"`
	TempleCode     string  `db:"temple_code" json:"templeCode"`
	Position       string  `db:"position" json:"position"`
	Sect           string  `db:"sect" json:"sect"`
	Type           string  `db:"type" json:"type"` // 佛教/道教
	AuthStatus     string  `db:"auth_status" json:"authStatus"`
	ShelfStatus    string  `db:"shelf_status" json:"shelfStatus"`
	PlatformStatus string  `db:"platform_status" json:"platformStatus"`
	Specialties    string  `db:"specialties" json:"specialties"` // 逗号分隔
	Avatar         string  `db:"avatar" json:"avatar"`
	Rating         float64 `db:"rating" json:"rating"`
	CreateTime     string  `db:"create_time" json:"createTime"`
	UpdateTime     string  `db:"update_time" json:"updateTime"`
}

// MasterModel 法师模型接口
type MasterModel interface {
	FindOne(ctx context.Context, id int64) (*Master, error)
	FindByCode(ctx context.Context, code string) (*Master, error)
	// FindList 寺院查询本寺院法师列表（按 templeCode + shelfStatus 筛选）
	FindList(ctx context.Context, templeCode, shelfStatus string, page, size int) ([]*Master, int64, error)
	// FindAll 平台查询所有法师（按 shelfStatus 筛选，空则全部）
	FindAll(ctx context.Context, shelfStatus string, page, size int) ([]*Master, int64, error)
	// FindCList C端公开列表（仅 on_shelf + normal），按 sect/type/templeCode 筛选
	FindCList(ctx context.Context, sect, mtype, templeCode string, page, size int) ([]*Master, int64, error)
	FindTempleNameByCode(ctx context.Context, templeCode string) (string, error)
	Insert(ctx context.Context, data *Master) (int64, error)
	Update(ctx context.Context, data *Master) error
	UpdateShelfStatus(ctx context.Context, id int64, status string) error
	UpdatePlatformStatus(ctx context.Context, id int64, status string) error
	UpdateAuthStatus(ctx context.Context, code, status string) error
	// NextCode 生成下一个法师编码（基于 MAX(id)+1，格式 M00x）
	NextCode(ctx context.Context) (string, error)
}

type masterModel struct {
	conn  sqlx.SqlConn
	table string
}

// NewMasterModel 构造法师模型
func NewMasterModel(conn sqlx.SqlConn) MasterModel {
	return &masterModel{conn: conn, table: "master"}
}

func (m *masterModel) FindOne(ctx context.Context, id int64) (*Master, error) {
	var master Master
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = ?", masterRows, m.table)
	err := m.conn.QueryRowCtx(ctx, &master, query, id)
	if err != nil {
		return nil, err
	}
	return &master, nil
}

func (m *masterModel) FindByCode(ctx context.Context, code string) (*Master, error) {
	var master Master
	query := fmt.Sprintf("SELECT %s FROM %s WHERE code = ?", masterRows, m.table)
	err := m.conn.QueryRowCtx(ctx, &master, query, code)
	if err != nil {
		return nil, err
	}
	return &master, nil
}

func (m *masterModel) FindList(ctx context.Context, templeCode, shelfStatus string, page, size int) ([]*Master, int64, error) {
	where := "WHERE temple_code = ?"
	args := []interface{}{templeCode}
	if shelfStatus != "" {
		where += " AND shelf_status = ?"
		args = append(args, shelfStatus)
	}
	return m.queryPage(ctx, where, args, "ORDER BY id DESC", page, size)
}

func (m *masterModel) FindAll(ctx context.Context, shelfStatus string, page, size int) ([]*Master, int64, error) {
	where := ""
	args := []interface{}{}
	if shelfStatus != "" {
		where = "WHERE shelf_status = ?"
		args = append(args, shelfStatus)
	}
	return m.queryPage(ctx, where, args, "ORDER BY id DESC", page, size)
}

func (m *masterModel) FindCList(ctx context.Context, sect, mtype, templeCode string, page, size int) ([]*Master, int64, error) {
	where := "WHERE shelf_status = ? AND platform_status = ?"
	args := []interface{}{MasterShelfStatusOnShelf, MasterPlatformStatusNormal}
	if sect != "" {
		where += " AND sect = ?"
		args = append(args, sect)
	}
	if mtype != "" {
		where += " AND type = ?"
		args = append(args, mtype)
	}
	if templeCode != "" {
		where += " AND temple_code = ?"
		args = append(args, templeCode)
	}
	return m.queryPage(ctx, where, args, "ORDER BY rating DESC, id DESC", page, size)
}

func (m *masterModel) FindTempleNameByCode(ctx context.Context, templeCode string) (string, error) {
	var name string
	err := m.conn.QueryRowCtx(ctx, &name, "SELECT name FROM temple WHERE code = ?", templeCode)
	return name, err
}

func (m *masterModel) queryPage(ctx context.Context, where string, args []interface{}, orderBy string, page, size int) ([]*Master, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	offset := (page - 1) * size

	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(1) FROM %s %s", m.table, where)
	err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*Master{}, 0, nil
	}

	var list []*Master
	listQuery := fmt.Sprintf("SELECT %s FROM %s %s %s LIMIT %d, %d", masterRows, m.table, where, orderBy, offset, size)
	err = m.conn.QueryRowsCtx(ctx, &list, listQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *masterModel) Insert(ctx context.Context, data *Master) (int64, error) {
	query := fmt.Sprintf("INSERT INTO %s (code, dharma_name, lay_name, temple_code, position, sect, type, auth_status, shelf_status, platform_status, specialties, avatar, rating) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)", m.table)
	res, err := m.conn.ExecCtx(ctx, query,
		data.Code, data.DharmaName, data.LayName, data.TempleCode, data.Position,
		data.Sect, data.Type, data.AuthStatus, data.ShelfStatus, data.PlatformStatus,
		data.Specialties, data.Avatar, data.Rating)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (m *masterModel) Update(ctx context.Context, data *Master) error {
	query := fmt.Sprintf("UPDATE %s SET dharma_name = ?, lay_name = ?, position = ?, sect = ?, type = ?, specialties = ?, avatar = ? WHERE id = ?", m.table)
	_, err := m.conn.ExecCtx(ctx, query,
		data.DharmaName, data.LayName, data.Position, data.Sect, data.Type,
		data.Specialties, data.Avatar, data.Id)
	return err
}

func (m *masterModel) UpdateShelfStatus(ctx context.Context, id int64, status string) error {
	query := fmt.Sprintf("UPDATE %s SET shelf_status = ? WHERE id = ?", m.table)
	_, err := m.conn.ExecCtx(ctx, query, status, id)
	return err
}

func (m *masterModel) UpdatePlatformStatus(ctx context.Context, id int64, status string) error {
	query := fmt.Sprintf("UPDATE %s SET platform_status = ? WHERE id = ?", m.table)
	_, err := m.conn.ExecCtx(ctx, query, status, id)
	return err
}

func (m *masterModel) UpdateAuthStatus(ctx context.Context, code, status string) error {
	query := fmt.Sprintf("UPDATE %s SET auth_status = ? WHERE code = ?", m.table)
	_, err := m.conn.ExecCtx(ctx, query, status, code)
	return err
}

func (m *masterModel) NextCode(ctx context.Context) (string, error) {
	var nextId int64
	query := fmt.Sprintf("SELECT IFNULL(MAX(id), 0) + 1 FROM %s", m.table)
	err := m.conn.QueryRowCtx(ctx, &nextId, query)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("M%03d", nextId), nil
}

// masterRows 法师表查询字段
const masterRows = "id, code, dharma_name, lay_name, temple_code, position, sect, type, auth_status, shelf_status, platform_status, specialties, avatar, rating, create_time, update_time"
