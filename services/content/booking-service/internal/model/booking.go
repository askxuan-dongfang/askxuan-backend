package model

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// 预约状态常量（与 state-machines.md §1 对齐：snake_case）
const (
	StatusPending    = "pending"     // 待确认
	StatusConfirmed  = "confirmed"   // 已确认
	StatusInProgress = "in_progress" // 进行中
	StatusCompleted  = "completed"   // 已完成
	StatusCancelled  = "cancelled"   // 已取消
	StatusReviewed   = "reviewed"    // 已评价（终态）
)

// validTransitions 合法的状态流转映射
// key = 当前状态，value = 允许流转到的目标状态集合
var validTransitions = map[string]map[string]bool{
	StatusPending: {
		StatusConfirmed: true,
		StatusCancelled: true,
	},
	StatusConfirmed: {
		StatusInProgress: true,
		StatusCancelled: true,
	},
	StatusInProgress: {
		StatusCompleted: true,
		StatusCancelled: true,
	},
	StatusCompleted: {
		StatusReviewed: true, // 用户提交评价 → reviewed
	},
	// reviewed / cancelled 为终态，不可再流转
}

// CanTransit 校验状态流转是否合法
func CanTransit(from, to string) bool {
	if from == to {
		return false
	}
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	return allowed[to]
}

// IsTerminalStatus 是否终态
func IsTerminalStatus(s string) bool {
	return s == StatusReviewed || s == StatusCancelled
}

// Booking 预约结构体
type Booking struct {
	Id             string  `db:"booking_no" json:"id"`
	UserId         string  `db:"user_id" json:"userId"`
	TempleId       string  `db:"temple_code" json:"templeId"`
	TempleName     string  `db:"temple_name" json:"templeName"`
	MasterId       string  `db:"master_code" json:"masterId"`
	MasterName     string  `db:"master_name" json:"masterName"`
	ServiceId      string  `db:"service_code" json:"serviceId"`
	ServiceName    string  `db:"service_name" json:"serviceName"`
	BookingDate    string  `db:"booking_date" json:"bookingDate"`
	TimeSlot       string  `db:"time_slot" json:"timeSlot"`
	MeritMoney     float64 `db:"merit_money" json:"meritMoney"`
	MeritMoneyTier string  `db:"merit_money_tier" json:"meritMoneyTier"`
	Status         string  `db:"status" json:"status"`
	Note           string  `db:"note" json:"note"`
	CreatedAt      string  `db:"create_time" json:"createdAt"`
}

// BookingModel 预约模型接口
type BookingModel interface {
	Insert(ctx context.Context, data *Booking) (*Booking, error)
	FindOne(ctx context.Context, bookingNo string) (*Booking, error)
	FindList(ctx context.Context, userId, status, templeId string, page, size int) ([]*Booking, int64, error)
	UpdateStatus(ctx context.Context, bookingNo, newStatus string) (*Booking, error)
	FindAdminList(ctx context.Context, templeId, status, masterId string, page, size int) ([]*Booking, int64, error)
}

type defaultBookingModel struct {
	conn sqlx.SqlConn
}

// NewBookingModel 构造预约模型
func NewBookingModel(conn sqlx.SqlConn) BookingModel {
	return &defaultBookingModel{conn: conn}
}

// 查询字段：booking 表字段 + JOIN temple/master/service_type 获取名称
const (
	bookingSelect = `b.booking_no, b.user_id, b.temple_code, t.name AS temple_name, ` +
		`b.master_code, m.dharma_name AS master_name, b.service_code, s.name AS service_name, ` +
		`b.booking_date, b.time_slot, b.merit_money, b.merit_money_tier, b.status, b.note, b.create_time`
	bookingJoins = ` LEFT JOIN temple t ON t.code = b.temple_code ` +
		`LEFT JOIN master m ON m.code = b.master_code ` +
		`LEFT JOIN service_type s ON s.code = b.service_code`
)

// Insert 新建预约，返回带 booking_no 与初始状态的对象
func (m *defaultBookingModel) Insert(ctx context.Context, data *Booking) (*Booking, error) {
	// 单号格式 B + yyyyMMddHHmmss + 3位毫秒，保证唯一性
	now := time.Now()
	bookingNo := fmt.Sprintf("B%s%03d", now.Format("20060102150405"), now.Nanosecond()/1e6)
	if data.Status == "" {
		data.Status = StatusPending
	}
	data.Id = bookingNo
	data.CreatedAt = now.Format("2006-01-02 15:04:05")

	const query = `INSERT INTO booking (booking_no, user_id, temple_code, master_code, service_code, booking_date, time_slot, merit_money, merit_money_tier, status, note, create_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := m.conn.ExecCtx(ctx, query, bookingNo, data.UserId, data.TempleId, data.MasterId,
		data.ServiceId, data.BookingDate, data.TimeSlot, data.MeritMoney, data.MeritMoneyTier,
		data.Status, data.Note, data.CreatedAt)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// FindOne 按单号查询预约（含寺院/法师/服务名称）
func (m *defaultBookingModel) FindOne(ctx context.Context, bookingNo string) (*Booking, error) {
	var b Booking
	query := fmt.Sprintf("SELECT %s FROM booking b %s WHERE b.booking_no = ?", bookingSelect, bookingJoins)
	err := m.conn.QueryRowCtx(ctx, &b, query, bookingNo)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// FindList 查询预约列表，支持按 userId / status / templeId 筛选 + 分页
func (m *defaultBookingModel) FindList(ctx context.Context, userId, status, templeId string, page, size int) ([]*Booking, int64, error) {
	where, args := buildWhere(userId, status, templeId, "")

	// 总数
	countQuery := fmt.Sprintf("SELECT COUNT(1) FROM booking b WHERE %s", where)
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*Booking{}, 0, nil
	}

	// 列表
	offset := (page - 1) * size
	listQuery := fmt.Sprintf("SELECT %s FROM booking b %s WHERE %s ORDER BY b.create_time DESC LIMIT ?, ?",
		bookingSelect, bookingJoins, where)
	listArgs := append(args, offset, size)
	var list []*Booking
	if err := m.conn.QueryRowsCtx(ctx, &list, listQuery, listArgs...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// UpdateStatus 更新预约状态（调用方需先校验 CanTransit），返回更新后的预约
func (m *defaultBookingModel) UpdateStatus(ctx context.Context, bookingNo, newStatus string) (*Booking, error) {
	const query = `UPDATE booking SET status = ? WHERE booking_no = ?`
	_, err := m.conn.ExecCtx(ctx, query, newStatus, bookingNo)
	if err != nil {
		return nil, err
	}
	return m.FindOne(ctx, bookingNo)
}

// FindAdminList 寺院管理台查询预约列表，按 temple_id 过滤，支持 status/masterId 筛选 + 分页
func (m *defaultBookingModel) FindAdminList(ctx context.Context, templeId, status, masterId string, page, size int) ([]*Booking, int64, error) {
	where, args := buildWhere("", status, templeId, masterId)

	countQuery := fmt.Sprintf("SELECT COUNT(1) FROM booking b WHERE %s", where)
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*Booking{}, 0, nil
	}

	offset := (page - 1) * size
	listQuery := fmt.Sprintf("SELECT %s FROM booking b %s WHERE %s ORDER BY b.create_time DESC LIMIT ?, ?",
		bookingSelect, bookingJoins, where)
	listArgs := append(args, offset, size)
	var list []*Booking
	if err := m.conn.QueryRowsCtx(ctx, &list, listQuery, listArgs...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// buildWhere 构建动态 WHERE 子句
func buildWhere(userId, status, templeId, masterId string) (string, []interface{}) {
	where := "1=1"
	var args []interface{}
	if userId != "" {
		where += " AND b.user_id = ?"
		args = append(args, userId)
	}
	if status != "" {
		where += " AND b.status = ?"
		args = append(args, status)
	}
	if templeId != "" {
		where += " AND b.temple_code = ?"
		args = append(args, templeId)
	}
	if masterId != "" {
		where += " AND b.master_code = ?"
		args = append(args, masterId)
	}
	return where, args
}
