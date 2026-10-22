// Package scheduling defines the shared master-calendar contract used by master and booking services.
package scheduling

import (
	"context"
	"encoding/json"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"sort"
	"strings"
	"time"
)

var ErrConflict = &common.BizError{Code: 40921, Msg: "排班已更新，请刷新后重试"}
var ErrUnavailable = &common.BizError{Code: 40922, Msg: "法师该时段不可预约，请重新选择"}

func ValidDate(date string) bool {
	t, e := time.Parse("2006-01-02", date)
	return e == nil && t.Format("2006-01-02") == date
}
func Range(slot string) (string, string, bool) {
	p := strings.Split(slot, "-")
	if len(p) != 2 {
		return "", "", false
	}
	for _, v := range p {
		t, e := time.Parse("15:04", v)
		if e != nil || t.Format("15:04") != v {
			return "", "", false
		}
	}
	return p[0], p[1], p[0] < p[1]
}
func Validate(date, status string, slots []string) error {
	if !ValidDate(date) || (status != "available" && status != "off") || len(slots) > 12 {
		return common.ErrParam
	}
	if status == "off" && len(slots) > 0 {
		return common.ErrParam
	}
	if status == "available" && len(slots) == 0 {
		return common.NewBizError(40001, "请添加可预约时段，或设为休息日")
	}
	ordered := append([]string{}, slots...)
	sort.Strings(ordered)
	last := ""
	for _, s := range ordered {
		a, b, ok := Range(s)
		if !ok || a < last {
			return common.NewBizError(40001, "时段格式不正确或互相重叠")
		}
		last = b
	}
	return nil
}

// No row keeps the existing service timetable. An explicit row is authoritative.
func Allows(status string, slots []string, requested string) bool {
	if status != "available" {
		return false
	}
	a, b, ok := Range(requested)
	if !ok {
		return false
	}
	for _, s := range slots {
		x, y, ok := Range(s)
		if ok && x <= a && b <= y {
			return true
		}
	}
	return false
}
func Read(ctx context.Context, q sqlx.Session, master, date string) (string, []string, error) {
	var v struct {
		Status string `db:"status"`
		Slots  string `db:"time_slots"`
	}
	err := q.QueryRowCtx(ctx, &v, `SELECT status,time_slots FROM askxuan_master.master_schedule WHERE master_code=? AND schedule_date=?`, master, date)
	if err == sqlx.ErrNotFound {
		return "", nil, nil
	}
	if err != nil {
		return "", nil, err
	}
	var slots []string
	if json.Unmarshal([]byte(v.Slots), &slots) != nil {
		return "", nil, common.ErrSystem
	}
	return v.Status, slots, nil
}

// The dedicated row is shared by calendar edits and booking reservations, including
// dates with no calendar yet. Booking needs no write or locking privilege on profiles.
func LockMaster(ctx context.Context, q sqlx.Session, master string) error {
	// Acquire the write lock before the first consistent read. Under REPEATABLE READ,
	// reading the profile first could create a snapshot predating a competing writer.
	if _, err := q.ExecCtx(ctx, `INSERT INTO askxuan_master.master_schedule_guard(master_code) VALUES(?) ON DUPLICATE KEY UPDATE master_code=VALUES(master_code)`, master); err != nil {
		return err
	}
	var id int64
	return q.QueryRowCtx(ctx, &id, `SELECT id FROM askxuan_master.master WHERE code=?`, master)
}
func Check(ctx context.Context, q sqlx.Session, master, date, slot string) error {
	if master == "" {
		return nil
	}
	status, slots, err := Read(ctx, q, master, date)
	if err != nil {
		return err
	}
	if status != "" && !Allows(status, slots, slot) {
		return ErrUnavailable
	}
	return nil
}
