package model

import (
	"reflect"
	"testing"
)

func TestPaidBookingReportWhereRequiresPaidNonCancelledAndInclusiveEndDate(t *testing.T) {
	where, args := paidBookingReportWhere("T001", "2026-09-01", "2026-09-03")
	wantWhere := "temple_code=? AND payment_status='success' AND status<>'cancelled' AND create_time>=? AND create_time<DATE_ADD(?,INTERVAL 1 DAY)"
	wantArgs := []interface{}{"T001", "2026-09-01 00:00:00", "2026-09-03 00:00:00"}
	if where != wantWhere {
		t.Fatalf("where=%q want=%q", where, wantWhere)
	}
	if !reflect.DeepEqual(args, wantArgs) {
		t.Fatalf("args=%v want=%v", args, wantArgs)
	}
}
