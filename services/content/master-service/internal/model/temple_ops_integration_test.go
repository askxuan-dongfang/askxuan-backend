package model

import (
	"context"
	"github.com/askxuan/common/scheduling"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestSharedCalendarRevisionIntegration(t *testing.T) {
	dsn := os.Getenv("TEMPLE_OPS_MASTER_DSN")
	if dsn == "" {
		t.Skip("requires task-only MySQL")
	}
	if !strings.Contains(dsn, "@tcp(127.0.0.1:63336)/askxuan_master?") {
		t.Fatal("refuse non-isolated database")
	}
	sqlx.DisableLog()
	db := sqlx.NewMysql(dsn)
	ctx := context.Background()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := db.ExecCtx(ctx, q, args...); e != nil {
			t.Fatal(e)
		}
	}
	exec(`INSERT IGNORE INTO master(code,dharma_name,temple_code) VALUES('MOPS_CAL','排班版本测试','T_OPS')`)
	exec(`DELETE FROM master_schedule WHERE master_code='MOPS_CAL'`)
	t.Cleanup(func() {
		exec(`DELETE FROM master_schedule WHERE master_code='MOPS_CAL'`)
		exec(`DELETE FROM master WHERE code='MOPS_CAL'`)
	})
	m := NewMasterScheduleModel(db)
	zero := int64(0)
	_, e := m.Upsert(ctx, &MasterSchedule{MasterCode: "MOPS_CAL", ScheduleDate: "2099-10-22", TimeSlots: `["09:00-12:00"]`, Status: "available"}, &zero)
	if e != nil {
		t.Fatal(e)
	}
	list, _, e := m.FindByMasterId(ctx, "MOPS_CAL", "2099-10-22", 1, 10)
	if e != nil || len(list) != 1 || list[0].Revision != 1 {
		t.Fatal("read revision", e, list)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	one := int64(1)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, e := m.Upsert(ctx, &MasterSchedule{MasterCode: "MOPS_CAL", ScheduleDate: "2099-10-22", TimeSlots: `[]`, Status: "off"}, &one)
			results <- e
		}()
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for e := range results {
		if e == nil {
			success++
		} else if e == scheduling.ErrConflict {
			conflict++
		} else {
			t.Fatal(e)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("lost update: %d %d", success, conflict)
	}
	if _, e = m.Upsert(ctx, &MasterSchedule{MasterCode: "MOPS_CAL", ScheduleDate: "2099-10-22", TimeSlots: `["09:00-12:00"]`, Status: "available"}); e != scheduling.ErrConflict {
		t.Fatal("unversioned client overwrote calendar", e)
	}
}
