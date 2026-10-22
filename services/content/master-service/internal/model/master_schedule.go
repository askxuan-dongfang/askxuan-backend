package model

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/askxuan/common/scheduling"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ============ 法师日程状态常量 ============

const (
	ScheduleStatusAvailable = "available" // 可预约
	ScheduleStatusBooked    = "booked"    // 已预约
	ScheduleStatusOff       = "off"       // 休息
)

// MasterSchedule 法师排班（对应 master_schedule 表）
type MasterSchedule struct {
	Revision     int64  `db:"revision" json:"revision"`
	Id           int64  `db:"id" json:"id"`
	MasterCode   string `db:"master_code" json:"masterCode"`
	ScheduleDate string `db:"schedule_date" json:"date"`   // 日期 YYYY-MM-DD
	TimeSlots    string `db:"time_slots" json:"timeSlots"` // JSON 数组字符串
	Status       string `db:"status" json:"status"`
	CreateTime   string `db:"create_time" json:"createTime"`
	UpdateTime   string `db:"update_time" json:"updateTime"`
}

// MasterScheduleModel 法师排班模型接口
type MasterScheduleModel interface {
	FindByMasterId(ctx context.Context, masterCode, date string, page, size int) ([]*MasterSchedule, int64, error)
	BatchUpsert(ctx context.Context, schedules []*MasterSchedule) error
	Upsert(ctx context.Context, schedule *MasterSchedule, expected ...*int64) (int64, error)
}

type masterScheduleModel struct {
	conn  sqlx.SqlConn
	table string
}

// NewMasterScheduleModel 构造法师排班模型
func NewMasterScheduleModel(conn sqlx.SqlConn) MasterScheduleModel {
	return &masterScheduleModel{conn: conn, table: "master_schedule"}
}

const masterScheduleRows = "id, master_code, DATE_FORMAT(schedule_date,'%Y-%m-%d') schedule_date, time_slots, status, revision, create_time, update_time"

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
func (m *masterScheduleModel) Upsert(ctx context.Context, schedule *MasterSchedule, expected ...*int64) (int64, error) {
	var revision *int64
	if len(expected) > 0 {
		revision = expected[0]
	}
	var slots []string
	if json.Unmarshal([]byte(schedule.TimeSlots), &slots) != nil {
		return 0, scheduling.ErrConflict
	}
	if err := scheduling.Validate(schedule.ScheduleDate, schedule.Status, slots); err != nil {
		return 0, err
	}
	var id int64
	err := m.conn.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		if err := scheduling.LockMaster(ctx, tx, schedule.MasterCode); err != nil {
			return err
		}
		var old struct {
			ID       int64 `db:"id"`
			Revision int64 `db:"revision"`
		}
		err := tx.QueryRowCtx(ctx, &old, `SELECT id,revision FROM master_schedule WHERE master_code=? AND schedule_date=? FOR UPDATE`, schedule.MasterCode, schedule.ScheduleDate)
		if err == sqlx.ErrNotFound {
			if revision != nil && *revision != 0 {
				return scheduling.ErrConflict
			}
			res, e := tx.ExecCtx(ctx, `INSERT INTO master_schedule(master_code,schedule_date,time_slots,status,revision) VALUES(?,?,?,?,1)`, schedule.MasterCode, schedule.ScheduleDate, schedule.TimeSlots, schedule.Status)
			if e != nil {
				return e
			}
			id, e = res.LastInsertId()
			return e
		}
		if err != nil {
			return err
		}
		if revision == nil || *revision != old.Revision {
			return scheduling.ErrConflict
		}
		_, err = tx.ExecCtx(ctx, `UPDATE master_schedule SET time_slots=?,status=?,revision=revision+1 WHERE id=?`, schedule.TimeSlots, schedule.Status, old.ID)
		id = old.ID
		return err
	})
	return id, err
}

// BatchUpsert uses the same validation and revision checks for every day.
func (m *masterScheduleModel) BatchUpsert(ctx context.Context, schedules []*MasterSchedule) error {
	for _, s := range schedules {
		if _, err := m.Upsert(ctx, s); err != nil {
			return err
		}
	}
	return nil
}
