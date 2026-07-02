package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ============ 法师日程状态常量 ============

const (
	ScheduleStatusAvailable = "available" // 可预约
	ScheduleStatusBooked    = "booked"    // 已预约
	ScheduleStatusOff       = "off"       // 休息
)

// MasterSchedule 法师排班（对应 askxuan_master.master_schedule）
type MasterSchedule struct {
	Id         int64  `db:"id" json:"id"`
	MasterCode string `db:"master_code" json:"masterCode"`
	ScheduleDate string `db:"schedule_date" json:"date"` // 日期 YYYY-MM-DD
	TimeSlots  string `db:"time_slots" json:"timeSlots"` // JSON 数组字符串
	Status     string `db:"status" json:"status"`
	CreateTime string `db:"create_time" json:"createTime"`
	UpdateTime string `db:"update_time" json:"updateTime"`
}

// MasterScheduleModel 法师排班模型接口
type MasterScheduleModel interface {
	FindByMasterId(ctx context.Context, masterCode, date string, page, size int) ([]*MasterSchedule, int64, error)
	BatchUpsert(ctx context.Context, schedules []*MasterSchedule) error
	Upsert(ctx context.Context, schedule *MasterSchedule) (int64, error)
}

type masterScheduleModel struct {
	conn  sqlx.SqlConn
	table string
}

// NewMasterScheduleModel 构造法师排班模型
func NewMasterScheduleModel(conn sqlx.SqlConn) MasterScheduleModel {
	return &masterScheduleModel{conn: conn, table: "askxuan_master.master_schedule"}
}

const masterScheduleRows = "id, master_code, schedule_date, time_slots, status, create_time, update_time"

func (m *masterScheduleModel) FindByMasterId(ctx context.Context, masterCode, date string, page, size int) ([]*MasterSchedule, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	offset := (page - 1) * size

	where := "WHERE master_code = ?"
	args := []interface{}{masterCode}
	if date != "" {
		where += " AND schedule_date = ?"
		args = append(args, date)
	}

	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(1) FROM %s %s", m.table, where)
	err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*MasterSchedule{}, 0, nil
	}

	var list []*MasterSchedule
	listQuery := fmt.Sprintf("SELECT %s FROM %s %s ORDER BY schedule_date ASC LIMIT %d, %d", masterScheduleRows, m.table, where, offset, size)
	err = m.conn.QueryRowsCtx(ctx, &list, listQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// Upsert 按 master_code + schedule_date 唯一键更新或插入
func (m *masterScheduleModel) Upsert(ctx context.Context, schedule *MasterSchedule) (int64, error) {
	query := fmt.Sprintf("INSERT INTO %s (master_code, schedule_date, time_slots, status) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE time_slots = VALUES(time_slots), status = VALUES(status)", m.table)
	res, err := m.conn.ExecCtx(ctx, query,
		schedule.MasterCode, schedule.ScheduleDate, schedule.TimeSlots, schedule.Status)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// BatchUpsert 批量更新或插入排班
func (m *masterScheduleModel) BatchUpsert(ctx context.Context, schedules []*MasterSchedule) error {
	if len(schedules) == 0 {
		return nil
	}
	query := fmt.Sprintf("INSERT INTO %s (master_code, schedule_date, time_slots, status) VALUES (?, ?, ?, ?) ON DUPLICATE KEY UPDATE time_slots = VALUES(time_slots), status = VALUES(status)", m.table)
	for _, s := range schedules {
		_, err := m.conn.ExecCtx(ctx, query, s.MasterCode, s.ScheduleDate, s.TimeSlots, s.Status)
		if err != nil {
			return err
		}
	}
	return nil
}
