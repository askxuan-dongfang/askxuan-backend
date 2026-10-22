package model

import (
	"context"
	"github.com/askxuan/common/scheduling"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"strings"
	"testing"
)

func TestTempleOpsReservationAndReportIntegration(t *testing.T) {
	dsn := os.Getenv("TEMPLE_OPS_BOOKING_DSN")
	if dsn == "" {
		t.Skip("requires task-only MySQL")
	}
	if !strings.Contains(dsn, "@tcp(127.0.0.1:63336)/askxuan_booking?") {
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
	exec(`INSERT IGNORE INTO askxuan_master.master(code,dharma_name,temple_code) VALUES('MOPS_TEST','排班测试','T_OPS')`)
	exec(`DELETE FROM booking WHERE request_id LIKE 'templeops-model-%'`)
	exec(`DELETE FROM booking_slot_inventory WHERE temple_code='T_OPS'`)
	exec(`DELETE FROM askxuan_master.master_schedule WHERE master_code='MOPS_TEST'`)
	t.Cleanup(func() {
		exec(`DELETE FROM booking WHERE request_id LIKE 'templeops-model-%'`)
		exec(`DELETE FROM booking_slot_inventory WHERE temple_code='T_OPS'`)
		exec(`DELETE FROM askxuan_master.master_schedule WHERE master_code='MOPS_TEST'`)
		exec(`DELETE FROM askxuan_master.master WHERE code='MOPS_TEST'`)
	})
	m := NewBookingModel(db)
	makeBooking := func(request, slot string) *Booking {
		return &Booking{RequestId: request, UserId: "991001", TempleId: "T_OPS", TempleName: "测试寺院", MasterId: "MOPS_TEST", MasterName: "排班测试", ServiceId: "S001", ServiceName: "测试服务", BookingDate: "2099-10-22", SlotCode: slot, TimeSlot: "09:00-10:00", ServiceFee: 20.25, MeritMoney: 100.50, TotalFee: 120.75, PriceSnapshot: "{}"}
	}
	exec(`INSERT INTO askxuan_master.master_schedule(master_code,schedule_date,time_slots,status,revision) VALUES('MOPS_TEST','2099-10-22','[]','off',1)`)
	if _, e := m.InsertWithReservation(ctx, makeBooking("templeops-model-off", "AM"), 5); e != scheduling.ErrUnavailable {
		t.Fatalf("rest day: %v", e)
	}
	exec(`UPDATE askxuan_master.master_schedule SET status='available',time_slots='["09:00-12:00"]' WHERE master_code='MOPS_TEST'`)
	first, e := m.InsertWithReservation(ctx, makeBooking("templeops-model-one", "AM"), 5)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.InsertWithReservation(ctx, makeBooking("templeops-model-overlap", "OTHER-SERVICE"), 5); e != scheduling.ErrUnavailable {
		t.Fatalf("overlapping master reservation: %v", e)
	}
	exec(`UPDATE booking SET payment_status='success',status='confirmed' WHERE booking_no=?`, first.Id)
	exec(`UPDATE askxuan_master.master_schedule SET status='off',time_slots='[]' WHERE master_code='MOPS_TEST'`)
	saved, e := m.FindOne(ctx, first.Id)
	if e != nil || saved.Status != "confirmed" {
		t.Fatal("calendar erased existing booking", e)
	}
	stats, _, _, _, e := m.Report(ctx, "T_OPS", "", "")
	if e != nil {
		t.Fatal(e)
	}
	if stats.ContractCents != 12075 || stats.TempleCents != 10050 || stats.MasterCents != 2025 || stats.BookingCount != 1 {
		t.Fatalf("wrong accounting: %+v", stats)
	}
	exec(`UPDATE booking SET payment_status='refunding',status='cancelled' WHERE booking_no=?`, first.Id)
	stats, _, _, _, e = m.Report(ctx, "T_OPS", "", "")
	if e != nil || stats.ContractCents != 0 || stats.BookingCount != 0 {
		t.Fatal("cancelled/refunding included", e, stats)
	}
	stats, _, _, _, e = m.Report(ctx, "OTHER_TEMPLE", "", "")
	if e != nil || stats.BookingCount != 0 {
		t.Fatal("tenant leak", e)
	}
}
