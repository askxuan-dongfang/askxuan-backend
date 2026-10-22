package logic

import (
	"context"
	"github.com/askxuan/booking-service/internal/model"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestTempleOpsExceptionsIntegration(t *testing.T) {
	dsn := os.Getenv("TEMPLE_OPS_BOOKING_DSN")
	if dsn == "" {
		t.Skip("requires task-only MySQL")
	}
	if !strings.Contains(dsn, "@tcp(127.0.0.1:63336)/askxuan_booking?") {
		t.Fatal("refuse non-isolated database")
	}
	sqlx.DisableLog()
	db := sqlx.NewMysql(dsn)
	base := context.Background()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := db.ExecCtx(base, q, args...); e != nil {
			t.Fatal(e)
		}
	}
	exec(`DELETE FROM booking WHERE request_id LIKE 'templeops-exc-%'`)
	exec(`DELETE FROM temple_exception_note WHERE temple_code='T_OPS_EX'`)
	t.Cleanup(func() {
		exec(`DELETE FROM temple_exception_note WHERE temple_code='T_OPS_EX'`)
		exec(`DELETE FROM booking WHERE request_id LIKE 'templeops-exc-%'`)
		exec(`DELETE FROM booking_slot_inventory WHERE temple_code='T_OPS_EX'`)
	})
	s := &svc.ServiceContext{DB: db}
	m := model.NewBookingModel(db)
	b, e := m.InsertWithReservation(base, &model.Booking{RequestId: "templeops-exc-one", UserId: "991001", TempleId: "T_OPS_EX", ServiceId: "S001", ServiceName: "异常测试", BookingDate: "2099-10-22", SlotCode: "AM", TimeSlot: "09:00-10:00", PriceSnapshot: "{}"}, 5)
	if e != nil {
		t.Fatal(e)
	}
	exec(`UPDATE booking SET status='cancelled',payment_status='refunding' WHERE booking_no=?`, b.Id)
	c := &common.CustomClaims{UserId: 991001, UserType: "admin", Roles: []string{"temple_reception"}, TempleCode: "T_OPS_EX"}
	ctx := context.WithValue(base, middleware.CtxKeyAdminClaims, c)
	out, e := TempleExceptions(ctx, s, "", false, 1)
	if e != nil {
		t.Fatal(e)
	}
	rows := out["list"].([]TempleException)
	if len(rows) != 1 || rows[0].Reason != "refund_pending" {
		t.Fatal("derived state", rows)
	}
	id := "booking:" + b.Id
	var wg sync.WaitGroup
	failures := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			failures <- AddExceptionNote(ctx, s, id, "templeops-request-0001", "escalated", "已与信众核对，提请平台跟进退款")
		}()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal("concurrent retry", err)
		}
	}
	if e := AddExceptionNote(ctx, s, id, "templeops-request-0001", "following", "重复请求号不得覆盖原始跟进"); e == nil {
		t.Fatal("idempotency key accepted a different payload")
	}
	notes, e := ExceptionNotes(ctx, s, id, false)
	if e != nil || len(notes) != 1 {
		t.Fatal("duplicate notes", e, len(notes))
	}
	other := *c
	other.TempleCode = "OTHER"
	otherCtx := context.WithValue(base, middleware.CtxKeyAdminClaims, &other)
	if _, e := ExceptionNotes(otherCtx, s, id, false); e == nil {
		t.Fatal("cross-tenant notes disclosed")
	}
	if e := AddExceptionNote(otherCtx, s, id, "templeops-request-0002", "following", "越权"); e == nil {
		t.Fatal("cross-tenant note write")
	}
	platform := context.WithValue(base, middleware.CtxKeyAdminClaims, &common.CustomClaims{UserId: 1, UserType: "admin", Roles: []string{"platform_super"}})
	if notes, e := ExceptionNotes(platform, s, id, true); e != nil || len(notes) != 1 {
		t.Fatal("platform cannot trace escalation", e)
	}
	saved, e := m.FindOne(base, b.Id)
	if e != nil || saved.PaymentStatus != "refunding" || saved.Status != "cancelled" {
		t.Fatal("note changed business state", e)
	}
	exec(`UPDATE booking SET payment_status='refunded' WHERE booking_no=?`, b.Id)
	out, e = TempleExceptions(ctx, s, "", false, 1)
	if e != nil || out["total"].(int64) != 0 {
		t.Fatal("resolved exception not removed", e)
	}
	if notes, e := ExceptionNotes(ctx, s, id, false); e != nil || len(notes) != 1 {
		t.Fatal("resolution erased audit", e)
	}
}
