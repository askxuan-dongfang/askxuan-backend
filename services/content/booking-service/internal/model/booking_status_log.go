package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ============ 预约状态变更日志 MySQL 存储 ============

// 操作人类型常量
const (
	OperatorTypeUser        = "user"         // 用户操作
	OperatorTypeTempleAdmin = "temple_admin" // 寺院管理员操作
	OperatorTypeSystem      = "system"       // 系统自动（超时取消）
)

// statusLogTable 状态日志表（位于 askxuan_booking 库）
const statusLogTable = "askxuan_booking.booking_status_log"

// BookingStatusLog 状态变更日志
type BookingStatusLog struct {
	Id           int64  `db:"id" json:"id"`
	BookingId    string `db:"booking_id" json:"bookingId"`
	FromStatus   string `db:"from_status" json:"fromStatus"`
	ToStatus     string `db:"to_status" json:"toStatus"`
	OperatorId   string `db:"operator_id" json:"operatorId"`
	OperatorType string `db:"operator_type" json:"operatorType"`
	Remark       string `db:"remark" json:"remark"`
	CreateTime   string `db:"create_time" json:"createTime"`
}

// BookingStatusLogModel 状态日志模型接口
type BookingStatusLogModel interface {
	Insert(ctx context.Context, data *BookingStatusLog) error
	FindByBookingId(ctx context.Context, bookingId string) ([]*BookingStatusLog, error)
}

type defaultBookingStatusLogModel struct {
	conn sqlx.SqlConn
}

// NewBookingStatusLogModel 构造状态日志模型
func NewBookingStatusLogModel(conn sqlx.SqlConn) BookingStatusLogModel {
	return &defaultBookingStatusLogModel{conn: conn}
}

// Insert 记录状态变更日志
func (m *defaultBookingStatusLogModel) Insert(ctx context.Context, data *BookingStatusLog) error {
	query := fmt.Sprintf(
		"INSERT INTO %s (booking_id, from_status, to_status, operator_id, operator_type, remark, create_time) VALUES (?, ?, ?, ?, ?, ?, NOW())",
		statusLogTable)
	_, err := m.conn.ExecCtx(ctx, query, data.BookingId, data.FromStatus, data.ToStatus,
		data.OperatorId, data.OperatorType, data.Remark)
	return err
}

// FindByBookingId 按预约单号查询状态变更日志
func (m *defaultBookingStatusLogModel) FindByBookingId(ctx context.Context, bookingId string) ([]*BookingStatusLog, error) {
	query := fmt.Sprintf(
		"SELECT id, booking_id, from_status, to_status, operator_id, operator_type, remark, create_time FROM %s WHERE booking_id = ? ORDER BY create_time ASC",
		statusLogTable)
	var logs []*BookingStatusLog
	if err := m.conn.QueryRowsCtx(ctx, &logs, query, bookingId); err != nil {
		return nil, err
	}
	return logs, nil
}
