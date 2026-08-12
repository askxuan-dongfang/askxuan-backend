package model

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ============ 寺院自定义服务 MySQL 存储 ============

// 服务上下架状态常量
const (
	TempleServiceStatusOnShelf  = "on_shelf"  // 上架
	TempleServiceStatusOffShelf = "off_shelf" // 下架
)

// templeServiceTable 寺院服务表
const templeServiceTable = "temple_service"

// TempleServiceRecord 寺院自定义服务（避免与 types.TempleService 混淆，model 层加 Record 后缀）
type TempleServiceRecord struct {
	Id          int64               `db:"id" json:"id"`
	TempleCode  string              `db:"temple_code" json:"templeCode"`
	ServiceCode string              `db:"service_code" json:"serviceCode"`
	ServiceName string              `db:"service_name" json:"serviceName"`
	Price       float64             `db:"price" json:"price"`
	TimeSlots   []string            `json:"timeSlots"` // DB 中为 JSON 字符串
	Slots       []TempleServiceSlot `json:"slots"`
	Status      string              `db:"status" json:"status"`
	CreateTime  string              `db:"create_time" json:"createTime"`
	UpdateTime  string              `db:"update_time" json:"updateTime"`
}

type TempleServiceSlot struct {
	Code      string `db:"slot_code" json:"code"`
	Label     string `db:"label" json:"label"`
	StartTime string `db:"start_time" json:"startTime"`
	EndTime   string `db:"end_time" json:"endTime"`
	Capacity  int    `db:"capacity" json:"capacity"`
	Status    string `db:"status" json:"status"`
	Sort      int    `db:"sort" json:"sort"`
}

// templeServiceRow 服务 DB 行结构（TimeSlots 为 JSON 字符串）
type templeServiceRow struct {
	Id          int64   `db:"id"`
	TempleCode  string  `db:"temple_code"`
	ServiceCode string  `db:"service_code"`
	ServiceName string  `db:"service_name"`
	Price       float64 `db:"price"`
	TimeSlots   string  `db:"time_slots"`
	Status      string  `db:"status"`
	CreateTime  string  `db:"create_time"`
	UpdateTime  string  `db:"update_time"`
}

// TempleServiceModel 寺院服务模型接口
type TempleServiceModel interface {
	Insert(ctx context.Context, data *TempleServiceRecord) (int64, error)
	FindOne(ctx context.Context, id int64) (*TempleServiceRecord, error)
	FindByCodes(ctx context.Context, templeCode, serviceCode string) (*TempleServiceRecord, error)
	FindByTempleId(ctx context.Context, templeCode string) ([]*TempleServiceRecord, error)
	FindList(ctx context.Context, templeCode, status string, page, size int) ([]*TempleServiceRecord, int64, error)
	Update(ctx context.Context, data *TempleServiceRecord) error
	UpdateStatus(ctx context.Context, id int64, status string) error
	FindIntentTags(ctx context.Context, id int64) ([]string, error)
	ReplaceIntentTags(ctx context.Context, id int64, tags []string) error
	ReplaceSlots(ctx context.Context, id int64, slots []TempleServiceSlot) error
	Delete(ctx context.Context, id int64) error
}

func ValidIntentTags(tags []string) bool {
	pattern := regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}$`)
	for _, raw := range tags {
		if !pattern.MatchString(strings.TrimSpace(raw)) {
			return false
		}
	}
	return true
}

type defaultTempleServiceModel struct {
	conn sqlx.SqlConn
}

// NewTempleServiceModel 构造寺院服务模型
func NewTempleServiceModel(conn sqlx.SqlConn) TempleServiceModel {
	return &defaultTempleServiceModel{conn: conn}
}

// Insert 创建寺院服务，返回自增 ID
func (m *defaultTempleServiceModel) Insert(ctx context.Context, data *TempleServiceRecord) (int64, error) {
	timeSlotsJSON := "[]"
	if len(data.TimeSlots) > 0 {
		if b, err := json.Marshal(data.TimeSlots); err == nil {
			timeSlotsJSON = string(b)
		}
	}
	if data.Status == "" {
		data.Status = TempleServiceStatusOnShelf
	}
	query := fmt.Sprintf(`INSERT INTO %s (temple_code, service_code, service_name, price, time_slots, status) VALUES (?, ?, ?, ?, ?, ?)`,
		templeServiceTable)
	res, err := m.conn.ExecCtx(ctx, query,
		data.TempleCode, data.ServiceCode, data.ServiceName,
		data.Price, timeSlotsJSON, data.Status)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// FindOne 按 ID 查询寺院服务
func (m *defaultTempleServiceModel) FindOne(ctx context.Context, id int64) (*TempleServiceRecord, error) {
	query := fmt.Sprintf(`SELECT id, temple_code, service_code, service_name, price, time_slots, status, create_time, update_time FROM %s WHERE id = ?`,
		templeServiceTable)
	var row templeServiceRow
	if err := m.conn.QueryRowCtx(ctx, &row, query, id); err != nil {
		return nil, err
	}
	service := rowToService(&row)
	service.Slots, _ = m.findSlots(ctx, service.Id)
	return service, nil
}

func (m *defaultTempleServiceModel) FindByCodes(ctx context.Context, templeCode, serviceCode string) (*TempleServiceRecord, error) {
	const query = `SELECT id, temple_code, service_code, service_name, price, time_slots, status, create_time, update_time FROM temple_service WHERE temple_code=? AND service_code=?`
	var row templeServiceRow
	if err := m.conn.QueryRowCtx(ctx, &row, query, templeCode, serviceCode); err != nil {
		return nil, err
	}
	service := rowToService(&row)
	service.Slots, _ = m.findSlots(ctx, service.Id)
	return service, nil
}

// FindByTempleId 查询寺院的服务列表（含下架）
func (m *defaultTempleServiceModel) FindByTempleId(ctx context.Context, templeCode string) ([]*TempleServiceRecord, error) {
	query := fmt.Sprintf(`SELECT id, temple_code, service_code, service_name, price, time_slots, status, create_time, update_time FROM %s WHERE temple_code = ? ORDER BY id ASC`,
		templeServiceTable)
	var rows []*templeServiceRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, templeCode); err != nil {
		return nil, err
	}
	list := make([]*TempleServiceRecord, 0, len(rows))
	for _, r := range rows {
		service := rowToService(r)
		service.Slots, _ = m.findSlots(ctx, service.Id)
		list = append(list, service)
	}
	return list, nil
}

// FindList 查询寺院服务列表，支持按 status 筛选 + 分页
func (m *defaultTempleServiceModel) FindList(ctx context.Context, templeCode, status string, page, size int) ([]*TempleServiceRecord, int64, error) {
	where := "1=1"
	var args []interface{}
	if templeCode != "" {
		where += " AND temple_code = ?"
		args = append(args, templeCode)
	}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(1) FROM %s WHERE %s`, templeServiceTable, where)
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*TempleServiceRecord{}, 0, nil
	}

	offset := (page - 1) * size
	listQuery := fmt.Sprintf(`SELECT id, temple_code, service_code, service_name, price, time_slots, status, create_time, update_time FROM %s WHERE %s ORDER BY id ASC LIMIT ?, ?`,
		templeServiceTable, where)
	listArgs := append(args, offset, size)
	var rows []*templeServiceRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, listQuery, listArgs...); err != nil {
		return nil, 0, err
	}
	list := make([]*TempleServiceRecord, 0, len(rows))
	for _, r := range rows {
		service := rowToService(r)
		service.Slots, _ = m.findSlots(ctx, service.Id)
		list = append(list, service)
	}
	return list, total, nil
}

// Update 更新寺院服务信息
func (m *defaultTempleServiceModel) Update(ctx context.Context, data *TempleServiceRecord) error {
	timeSlotsJSON := "[]"
	if len(data.TimeSlots) > 0 {
		if b, err := json.Marshal(data.TimeSlots); err == nil {
			timeSlotsJSON = string(b)
		}
	}
	query := fmt.Sprintf(`UPDATE %s SET temple_code = ?, service_code = ?, service_name = ?, price = ?, time_slots = ?, status = ? WHERE id = ?`,
		templeServiceTable)
	_, err := m.conn.ExecCtx(ctx, query,
		data.TempleCode, data.ServiceCode, data.ServiceName,
		data.Price, timeSlotsJSON, data.Status, data.Id)
	return err
}

// UpdateStatus 更新服务上下架状态
func (m *defaultTempleServiceModel) UpdateStatus(ctx context.Context, id int64, status string) error {
	query := fmt.Sprintf(`UPDATE %s SET status = ? WHERE id = ?`, templeServiceTable)
	_, err := m.conn.ExecCtx(ctx, query, status, id)
	return err
}

// Delete 删除寺院服务
func (m *defaultTempleServiceModel) Delete(ctx context.Context, id int64) error {
	return m.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		if _, err := session.ExecCtx(ctx, `DELETE FROM temple_service_slot WHERE temple_service_id=?`, id); err != nil {
			return err
		}
		_, err := session.ExecCtx(ctx, `DELETE FROM temple_service WHERE id=?`, id)
		return err
	})
}

func (m *defaultTempleServiceModel) findSlots(ctx context.Context, id int64) ([]TempleServiceSlot, error) {
	var slots []TempleServiceSlot
	if err := m.conn.QueryRowsCtx(ctx, &slots, `SELECT slot_code,label,start_time,end_time,capacity,status,sort FROM temple_service_slot WHERE temple_service_id=? ORDER BY sort,id`, id); err != nil {
		return nil, err
	}
	if slots == nil {
		slots = []TempleServiceSlot{}
	}
	return slots, nil
}

func (m *defaultTempleServiceModel) ReplaceSlots(ctx context.Context, id int64, slots []TempleServiceSlot) error {
	return m.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		if _, err := session.ExecCtx(ctx, `DELETE FROM temple_service_slot WHERE temple_service_id=?`, id); err != nil {
			return err
		}
		for i, slot := range slots {
			if slot.Code == "" {
				slot.Code = fmt.Sprintf("slot_%02d", i+1)
			}
			if slot.Label == "" {
				slot.Label = fmt.Sprintf("时段%d", i+1)
			}
			if slot.Capacity < 1 {
				slot.Capacity = 10
			}
			if slot.Status == "" {
				slot.Status = "enabled"
			}
			if slot.Sort == 0 {
				slot.Sort = i + 1
			}
			if slot.StartTime == "" || slot.EndTime == "" || slot.StartTime >= slot.EndTime {
				return fmt.Errorf("invalid service slot")
			}
			if _, err := session.ExecCtx(ctx, `INSERT INTO temple_service_slot(temple_service_id,slot_code,label,start_time,end_time,capacity,status,sort) VALUES(?,?,?,?,?,?,?,?)`, id, slot.Code, slot.Label, slot.StartTime, slot.EndTime, slot.Capacity, slot.Status, slot.Sort); err != nil {
				return err
			}
		}
		return nil
	})
}

func (m *defaultTempleServiceModel) FindIntentTags(ctx context.Context, id int64) ([]string, error) {
	var rows []struct {
		Code string `db:"tag_code"`
	}
	if err := m.conn.QueryRowsCtx(ctx, &rows, "SELECT tag_code FROM temple_service_intent_tag WHERE temple_service_id=? ORDER BY tag_code", id); err != nil {
		return nil, err
	}
	tags := make([]string, 0, len(rows))
	for _, row := range rows {
		tags = append(tags, row.Code)
	}
	return tags, nil
}

func (m *defaultTempleServiceModel) ReplaceIntentTags(ctx context.Context, id int64, tags []string) error {
	if !ValidIntentTags(tags) {
		return fmt.Errorf("invalid intent tags")
	}
	return m.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		if _, err := session.ExecCtx(ctx, "DELETE FROM temple_service_intent_tag WHERE temple_service_id=?", id); err != nil {
			return err
		}
		seen := map[string]bool{}
		for _, raw := range tags {
			code := strings.TrimSpace(raw)
			if seen[code] {
				continue
			}
			seen[code] = true
			if _, err := session.ExecCtx(ctx, "INSERT INTO temple_service_intent_tag(temple_service_id,tag_code) VALUES(?,?)", id, code); err != nil {
				return err
			}
		}
		return nil
	})
}

// rowToService 将 DB 行结构转为 TempleServiceRecord（TimeSlots JSON 反序列化）
func rowToService(row *templeServiceRow) *TempleServiceRecord {
	var slots []string
	if row.TimeSlots != "" {
		_ = json.Unmarshal([]byte(row.TimeSlots), &slots)
	}
	if slots == nil {
		slots = []string{}
	}
	return &TempleServiceRecord{
		Id:          row.Id,
		TempleCode:  row.TempleCode,
		ServiceCode: row.ServiceCode,
		ServiceName: row.ServiceName,
		Price:       row.Price,
		TimeSlots:   slots,
		Status:      row.Status,
		CreateTime:  row.CreateTime,
		UpdateTime:  row.UpdateTime,
	}
}
