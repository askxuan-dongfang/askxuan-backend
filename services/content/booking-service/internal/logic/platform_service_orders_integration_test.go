package logic

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/askxuan/booking-service/internal/model"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/booking-service/internal/types"
	"github.com/askxuan/common/middleware"
	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// A separate disposable schema is required. This test never drops a schema or
// table, and cleanup is limited to the unique fixture IDs created by this run.
func TestPlatformServiceOrdersIntegration(t *testing.T) {
	dsn := os.Getenv("PLATFORM_SERVICE_TEST_DSN")
	if dsn == "" {
		t.Skip("set PLATFORM_SERVICE_TEST_DSN to a disposable askxuan_admin_test_* database")
	}
	db := sqlx.NewMysql(dsn)
	sqlx.DisableLog()
	ctx := context.Background()
	var database string
	if err := db.QueryRowCtx(ctx, &database, "SELECT DATABASE()"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(database, "askxuan_admin_test_") {
		t.Fatal("refusing non-test database")
	}
	init, err := os.ReadFile("../../../../../db/init.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"booking", "booking_status_log", "booking_receipt", "booking_receipt_file", "booking_progress", "booking_progress_file"} {
		prefix := "CREATE TABLE IF NOT EXISTS " + table + " ("
		if table == "booking" {
			prefix = "CREATE TABLE `booking`"
		} else if table == "booking_status_log" {
			prefix = "CREATE TABLE IF NOT EXISTS `booking_status_log`"
		}
		start := strings.Index(string(init), prefix)
		if start < 0 {
			t.Fatalf("missing schema for %s", table)
		}
		ddl := strings.SplitN(string(init)[start:], ";", 2)[0]
		ddl = strings.Replace(ddl, "CREATE TABLE `booking`", "CREATE TABLE IF NOT EXISTS `booking`", 1)
		if _, err = db.ExecCtx(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	consult, err := os.ReadFile("../../../../../scripts/db/20260814_paid_consultation.sql")
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(consult), "CREATE TABLE IF NOT EXISTS askxuan_booking.consultation_order")
	if start < 0 {
		t.Fatal("missing consultation schema")
	}
	ddl := strings.ReplaceAll(strings.SplitN(string(consult)[start:], ";", 2)[0], "askxuan_booking.", "")
	if _, err = db.ExecCtx(ctx, ddl); err != nil {
		t.Fatal(err)
	}

	marker := fmt.Sprintf("AS%d", time.Now().UnixNano())
	b1, b2, b3, b4 := marker+"B1", marker+"B2", marker+"B3", marker+"B4"
	c1, c2 := marker+"C1", marker+"C2"
	receiptID, progressID := uuid.NewString(), uuid.NewString()
	t.Cleanup(func() {
		_, _ = db.ExecCtx(ctx, "DELETE FROM booking_progress_file WHERE progress_id=?", progressID)
		for _, table := range []string{"booking_receipt_file", "booking_receipt", "booking_progress"} {
			_, _ = db.ExecCtx(ctx, "DELETE FROM "+table+" WHERE booking_no IN (?,?,?,?)", b1, b2, b3, b4)
		}
		_, _ = db.ExecCtx(ctx, "DELETE FROM booking_status_log WHERE booking_id IN (?,?,?,?)", b1, b2, b3, b4)
		_, _ = db.ExecCtx(ctx, "DELETE FROM booking WHERE booking_no IN (?,?,?,?)", b1, b2, b3, b4)
		_, _ = db.ExecCtx(ctx, "DELETE FROM consultation_order WHERE order_no IN (?,?)", c1, c2)
	})
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := db.ExecCtx(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []struct{ id, temple, master, status, payment, created, service string }{
		{b1, "AST1", "ASM1", "pending", "success", "2026-10-20 00:00:00", "祈福_%"},
		{b2, "AST2", "ASM2", "pending_receipt", "success", "2026-10-21 23:59:59", "供灯"},
		{b4, "", "ASM2", "confirmed", "success", "2026-10-21 23:59:59", "开光"},
		{b3, "AST2", "ASM2", "cancelled", "refunded", "2026-10-22 00:00:00", "上香"},
	} {
		exec(`INSERT INTO booking(booking_no,user_id,temple_code,temple_name,master_code,master_name,service_code,service_name,booking_date,time_slot,service_fee,total_fee,status,payment_status,create_time) VALUES(?,99000001,?,?,?,?, 'ASSERVICE',?,'2026-10-25','09:00-10:00',88,88,?,?,?)`, row.id, row.temple, marker+"寺院", row.master, marker+"师傅", row.service, row.status, row.payment, row.created)
	}
	for i, id := range []string{c1, c2} {
		status, payment, channel := "active", "success", "demo_balance"
		if i == 1 {
			status, payment, channel = "pending_payment", "pending", ""
		}
		exec(`INSERT INTO consultation_order(order_no,request_id,user_id,master_code,master_name,temple_code,temple_name,consult_fee,valid_hours,response_minutes,question,payment_status,payment_channel,status,create_time) VALUES(?,?,99000001,'ASM1',?,'AST1',?,36,24,30,?,?,?,?,?)`, id, id, marker+"师傅", marker+"寺院", "下单问题，不是私聊正文", payment, channel, status, fmt.Sprintf("2026-10-2%d 10:00:00", i))
	}
	s := &svc.ServiceContext{DB: db, BookingModel: model.NewBookingModel(db), ConsultationModel: model.NewConsultationModel(db), StatusLogModel: model.NewBookingStatusLogModel(db)}
	super := serviceReaderContext("admin", 9, "platform_super")
	l := NewPlatformServiceOrdersLogic(super, s)
	list := func(req types.PlatformServiceOrderListReq, want int64, ids ...string) {
		t.Helper()
		req.Q = marker
		page, err := l.Bookings(&req)
		if err != nil || page.Total != want || len(page.List) != len(ids) {
			t.Fatalf("page count/length mismatch: total=%v want=%d err=%v", page, want, err)
		}
		for i, id := range ids {
			if page.List[i].Id != id {
				t.Fatalf("order=%s want=%s", page.List[i].Id, id)
			}
		}
	}
	list(types.PlatformServiceOrderListReq{Page: 1, Size: 2}, 4, b3, b4)
	list(types.PlatformServiceOrderListReq{Page: 2, Size: 2}, 4, b2, b1)
	list(types.PlatformServiceOrderListReq{Page: 3, Size: 2}, 4)
	list(types.PlatformServiceOrderListReq{TempleID: "AST2"}, 2, b3, b2)
	list(types.PlatformServiceOrderListReq{MasterID: "ASM2"}, 3, b3, b4, b2)
	list(types.PlatformServiceOrderListReq{From: "2026-10-21", To: "2026-10-21"}, 2, b4, b2)
	list(types.PlatformServiceOrderListReq{PaymentStatus: "refunded"}, 1, b3)
	list(types.PlatformServiceOrderListReq{Status: "pending_receipt"}, 1, b2)
	for _, literal := range []string{"%", "_"} {
		page, err := l.Bookings(&types.PlatformServiceOrderListReq{Q: literal, MasterID: "ASM1"})
		if err != nil || page.Total != 1 || page.List[0].Id != b1 {
			t.Fatalf("literal search %q: %v %v", literal, page, err)
		}
	}
	consultPage, err := l.Consultations(&types.PlatformServiceOrderListReq{Q: marker, Page: 1, Size: 1})
	if err != nil || consultPage.Total != 2 || len(consultPage.List) != 1 || consultPage.List[0].Id != c2 || consultPage.List[0].Question != "" {
		t.Fatalf("consultation real page/sensitive field: %v %v", consultPage, err)
	}
	consultPage, err = l.Consultations(&types.PlatformServiceOrderListReq{Q: marker, Status: "active", PaymentStatus: "success"})
	if err != nil || consultPage.Total != 1 || consultPage.List[0].Id != c1 || !consultPage.List[0].Simulated {
		t.Fatalf("consultation filters/simulated flag: %v %v", consultPage, err)
	}
	consultDetail, err := l.Consultation(c1)
	if err != nil || consultDetail.Question != "下单问题，不是私聊正文" || !consultDetail.Simulated {
		t.Fatalf("source consultation detail: %v %v", consultDetail, err)
	}
	bookingDetail, err := l.Booking(b2)
	if err != nil || bookingDetail.TempleId != "AST2" || bookingDetail.TotalFee != 88 {
		t.Fatalf("source booking detail: %v %v", bookingDetail, err)
	}
	for _, tc := range []struct{ kind, role string }{{"user", "customer"}, {"admin", "temple_admin"}, {"master", "master"}, {"admin", "service_reader"}, {"service", "platform_super"}} {
		denied := NewPlatformServiceOrdersLogic(serviceReaderContext(tc.kind, 99000001, tc.role), s)
		if _, err := denied.Bookings(&types.PlatformServiceOrderListReq{}); err == nil {
			t.Fatalf("%s acquired cross-provider listing", tc.role)
		}
		if _, err := denied.Consultation(c1); err == nil {
			t.Fatalf("%s acquired source consultation", tc.role)
		}
		if _, err := denied.Fulfillment(b2); err == nil {
			t.Fatalf("%s acquired cross-provider fulfillment", tc.role)
		}
	}

	exec(`INSERT INTO booking_status_log(booking_id,from_status,to_status,operator_id,operator_type,remark) VALUES(?,'in_progress','pending_receipt','777','temple_admin','已发布回执')`, b2)
	exec(`INSERT INTO booking_receipt(id,booking_no,summary,operator_id,operator_type,digest) VALUES(?,?,'完成记录','777','temple_admin',?)`, receiptID, b2, strings.Repeat("0", 64))
	exec(`INSERT INTO booking_progress(id,booking_no,kind,content,operator_id,operator_type) VALUES(?,?,'update','已发布服务进展','777','temple_admin')`, progressID, b2)
	t.Setenv("CHAT_FILES_DIR", t.TempDir())
	if err := os.MkdirAll(receiptDir(), 0700); err != nil {
		t.Fatal(err)
	}
	fileBytes := []byte("verified receipt fixture")
	hash := sha256.Sum256(fileBytes)
	files := []string{uuid.NewString(), uuid.NewString(), uuid.NewString()}
	for i, fileID := range files {
		publishedReceipt := ""
		if i == 0 {
			publishedReceipt = receiptID
		}
		exec(`INSERT INTO booking_receipt_file(id,booking_no,owner_id,receipt_id,name,content_type,file_size,sha256) VALUES(?,?,'777',?,'receipt.png','image/png',?,?)`, fileID, b2, publishedReceipt, len(fileBytes), hex.EncodeToString(hash[:]))
		if err := os.WriteFile(filepath.Join(receiptDir(), fileID), fileBytes, 0600); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO booking_progress_file(progress_id,file_id) VALUES(?,?)`, progressID, files[1])
	fulfillment, err := l.Fulfillment(b2)
	if err != nil {
		t.Fatal(err)
	}
	if len(fulfillment["logs"].([]*model.BookingStatusLog)) != 1 || len(fulfillment["receipts"].([]Receipt)) != 1 || len(fulfillment["records"].([]ProgressRecord)) != 1 {
		t.Fatal("fulfillment lost source logs, receipts or progress")
	}
	if len(fulfillment["receipts"].([]Receipt)[0].Files) != 1 || len(fulfillment["records"].([]ProgressRecord)[0].Files) != 1 {
		t.Fatal("fulfillment exposed draft uploads or lost published attachments")
	}
	for _, fileID := range files[:2] {
		w := httptest.NewRecorder()
		ReceiptDownload(w, httptest.NewRequest("GET", "/", nil).WithContext(super), s, b2, fileID)
		if w.Header().Get("Content-Type") != "image/png" || !bytes.Equal(w.Body.Bytes(), fileBytes) {
			t.Fatalf("platform_super cannot read published attachment: %s", w.Body.String())
		}
	}
	for _, tc := range []struct {
		booking, file string
		actor         context.Context
	}{{b2, files[2], super}, {b1, files[0], super}, {b2, files[0], serviceReaderContext("user", 99000002, "customer")}} {
		w := httptest.NewRecorder()
		ReceiptDownload(w, httptest.NewRequest("GET", "/", nil).WithContext(tc.actor), s, tc.booking, tc.file)
		var response struct {
			Code int `json:"code"`
		}
		if json.Unmarshal(w.Body.Bytes(), &response) != nil || response.Code == 0 {
			t.Fatal("draft, wrong-booking or non-owner receipt download accepted")
		}
	}
	if _, _, _, err := BookingAccess(super, s, b2, true); err == nil {
		t.Fatal("read-only super acquired provider execution permission")
	}
	// Existing owner/tenant checks remain in force even though the new platform
	// endpoint can read these same persisted orders.
	otherTemple := context.WithValue(serviceReaderContext("admin", 77, "temple_admin"), middleware.CtxKeyTempleCode, "AST1")
	if _, _, _, err := BookingAccess(otherTemple, s, b2, false); err == nil {
		t.Fatal("existing tenant boundary widened")
	}
	bookingDetail, err = l.Booking(b2)
	if err != nil || bookingDetail.Status != "pending_receipt" || bookingDetail.PaymentStatus != "success" {
		t.Fatal("read-only operations changed the source state")
	}
}
