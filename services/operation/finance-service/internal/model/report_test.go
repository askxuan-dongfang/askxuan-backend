package model

import (
	"reflect"
	"testing"
)

func TestReportTimeWhereUsesInclusiveCalendarDateRange(t *testing.T) {
	where, args := reportTimeWhere("create_time", "2026-09-01", " 2026-09-03 ")
	wantWhere := "1=1 AND create_time>=? AND create_time<DATE_ADD(?,INTERVAL 1 DAY)"
	wantArgs := []interface{}{"2026-09-01 00:00:00", "2026-09-03 00:00:00"}
	if where != wantWhere {
		t.Fatalf("where=%q want=%q", where, wantWhere)
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args=%v want=%v", args, wantArgs)
	}
}
