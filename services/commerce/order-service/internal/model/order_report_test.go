package model

import (
	"reflect"
	"testing"
)

func TestPaidOrderReportWhereUsesQualifiedColumnsAndInclusiveEndDate(t *testing.T) {
	where, args := paidOrderReportWhere("o.create_time", "o.status", " 2026-09-01 ", "2026-09-03")
	wantWhere := "o.status IN ('paid','shipped','completed','in_return') AND o.create_time>=? AND o.create_time<DATE_ADD(?,INTERVAL 1 DAY)"
	wantArgs := []interface{}{"2026-09-01 00:00:00", "2026-09-03 00:00:00"}
	if where != wantWhere {
		t.Fatalf("where=%q want=%q", where, wantWhere)
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args=%v want=%v", args, wantArgs)
	}
}
