package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/askxuan/booking-service/internal/model"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/booking-service/internal/types"
	"github.com/askxuan/common/middleware"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestFulfillmentIntegration(t *testing.T) {
	dsn := os.Getenv("FULFILLMENT_TEST_DSN")
	if dsn == "" {
		t.Skip("set FULFILLMENT_TEST_DSN to a disposable askxuan_fulfillment_test_* database")
	}
	db := sqlx.NewMysql(dsn)
	sqlx.DisableLog()
	ctx := context.Background()
	var database string
	if e := db.QueryRowCtx(ctx, &database, "SELECT DATABASE()"); e != nil {
		t.Fatal(e)
	}
	if !strings.HasPrefix(database, "askxuan_fulfillment_test_") {
		t.Fatal("refusing non-test database")
	}
	root := "../../../../.."
	init, e := os.ReadFile(root + "/db/init.sql")
	if e != nil {
		t.Fatal(e)
	}
	tables := []string{"event_outbox", "booking", "booking_status_log", "booking_slot_inventory", "booking_chat_message", "booking_receipt", "booking_receipt_file"}
	for _, table := range tables {
		source := string(init)
		prefix := "CREATE TABLE `" + table + "`"
		if table != "booking" {
			prefix = "CREATE TABLE IF NOT EXISTS `" + table + "`"
		}
		if table == "event_outbox" {
			prefix = "CREATE TABLE IF NOT EXISTS `askxuan_booking`.`event_outbox`"
		}
		if strings.HasPrefix(table, "booking_receipt") {
			prefix = "CREATE TABLE IF NOT EXISTS " + table + " ("
		}
		i := strings.Index(source, prefix)
		if i < 0 {
			t.Fatalf("missing table %s", table)
		}
		ddl := strings.ReplaceAll(strings.SplitN(source[i:], ";", 2)[0], "`askxuan_booking`.", "")
		if _, e = db.ExecCtx(ctx, ddl); e != nil {
			t.Fatal(e)
		}
	}
	consult, e := os.ReadFile(root + "/scripts/db/20260814_paid_consultation.sql")
	if e != nil {
		t.Fatal(e)
	}
	i := strings.Index(string(consult), "CREATE TABLE IF NOT EXISTS askxuan_booking.consultation_order")
	ddl := strings.ReplaceAll(strings.SplitN(string(consult)[i:], ";", 2)[0], "askxuan_booking.", "")
	if _, e = db.ExecCtx(ctx, ddl); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		for _, table := range append(tables, "consultation_order") {
			_, _ = db.ExecCtx(ctx, "DROP TABLE IF EXISTS "+table)
		}
	})
	t.Setenv("CHAT_FILES_DIR", t.TempDir())
	s := &svc.ServiceContext{DB: db, BookingModel: model.NewBookingModel(db), StatusLogModel: model.NewBookingStatusLogModel(db), ChatModel: model.NewBookingChatModel(db), MasterClient: chatTestMasters{}}
	contextFor := func(id int64, role, temple string, master int64) context.Context {
		c := context.WithValue(ctx, middleware.CtxKeyUserID, id)
		c = context.WithValue(c, middleware.CtxKeyRoles, []string{role})
		c = context.WithValue(c, middleware.CtxKeyTempleCode, temple)
		return context.WithValue(c, middleware.CtxKeyMasterID, master)
	}
	user := contextFor(98765001, "customer", "", 0)
	temple := contextFor(7, "temple_admin", "TTEST", 0)
	other := contextFor(8, "temple_admin", "OTHER", 0)
	intruder := contextFor(999, "customer", "", 0)
	master := contextFor(6, "master", "", 98765002)
	create := func(id, masterCode string) {
		_, e := db.ExecCtx(ctx, `INSERT INTO booking(booking_no,user_id,temple_code,temple_name,master_code,master_name,service_code,service_name,booking_date,time_slot,status,payment_status) VALUES(?,98765001,'TTEST','测试寺院',?,'测试大师','STEST','祈福',CURDATE(),'09:00-10:00','pending','success')`, id, masterCode)
		if e != nil {
			t.Fatal(e)
		}
	}
	create("TEST-TEMPLE", "")
	create("TEST-MASTER", "CHATTEST")
	if _, _, _, e := BookingAccess(other, s, "TEST-TEMPLE", true); e == nil {
		t.Fatal("cross temple access")
	}
	if _, _, _, e := BookingAccess(intruder, s, "TEST-TEMPLE", false); e == nil {
		t.Fatal("cross user access")
	}
	if _, _, _, e := BookingAccess(user, s, "TEST-TEMPLE", true); e == nil {
		t.Fatal("customer execution")
	}
	if _, _, _, e := BookingAccess(master, s, "TEST-TEMPLE", true); e == nil {
		t.Fatal("master acquired whole temple execution")
	}
	if _, _, _, e := BookingAccess(master, s, "TEST-MASTER", true); e != nil {
		t.Fatal(e)
	}
	list, total, e := s.ChatModel.ListConversations(ctx, "98765001", "", 1, 20, "", "u_98765001", false)
	if e != nil {
		t.Fatal(e)
	}
	if total != 1 || len(list) != 1 || list[0].BookingId != "TEST-MASTER" {
		t.Fatalf("whole temple chat leaked: %+v", list)
	}
	if e := TransitionWithAudit(ctx, s, "TEST-TEMPLE", model.StatusInProgress, "7", "temple_admin", ""); e == nil {
		t.Fatal("skipped confirmation")
	}
	if _, e := SubmitReceipt(temple, s, "TEST-TEMPLE", ReceiptRequest{Summary: "应拒绝过早提交", FileIds: []string{"none"}}); e == nil {
		t.Fatal("early receipt")
	}
	if e := TransitionWithAudit(ctx, s, "TEST-TEMPLE", model.StatusConfirmed, "7", "temple_admin", "确认接单"); e != nil {
		t.Fatal(e)
	}
	var wins atomic.Int32
	var wg sync.WaitGroup
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if TransitionWithAudit(ctx, s, "TEST-TEMPLE", model.StatusInProgress, "7", "temple_admin", "开始执行") == nil {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("concurrent starts %d", wins.Load())
	}
	if e := TransitionWithAudit(ctx, s, "TEST-TEMPLE", model.StatusCompleted, "7", "temple_admin", ""); e == nil {
		t.Fatal("completed without receipt")
	}
	upload := func(id string, provider context.Context) ReceiptFile {
		var media bytes.Buffer
		if e := png.Encode(&media, image.NewRGBA(image.Rect(0, 0, 4, 4))); e != nil {
			t.Fatal(e)
		}
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, _ := writer.CreateFormFile("file", "evidence.png")
		_, _ = part.Write(media.Bytes())
		_ = writer.Close()
		r := httptest.NewRequest("POST", "/", &body).WithContext(provider)
		r.Header.Set("Content-Type", writer.FormDataContentType())
		w := httptest.NewRecorder()
		ReceiptUpload(w, r, s, id)
		var response struct {
			Code int         `json:"code"`
			Data ReceiptFile `json:"data"`
		}
		if e := json.Unmarshal(w.Body.Bytes(), &response); e != nil || response.Code != 0 || response.Data.Id == "" {
			t.Fatalf("upload %s", w.Body.String())
		}
		return response.Data
	}
	file := upload("TEST-TEMPLE", temple)
	w := httptest.NewRecorder()
	ReceiptDownload(w, httptest.NewRequest("GET", "/", nil).WithContext(user), s, "TEST-TEMPLE", file.Id)
	if strings.HasPrefix(w.Header().Get("Content-Type"), "image/") {
		t.Fatal("draft exposed")
	}
	if _, e := SubmitReceipt(temple, s, "TEST-TEMPLE", ReceiptRequest{Summary: "没有回执附件不可提交"}); e == nil {
		t.Fatal("empty receipt accepted")
	}
	if _, e := SubmitReceipt(temple, s, "TEST-TEMPLE", ReceiptRequest{Summary: "第一版执行完成记录", FileIds: []string{file.Id, file.Id}}); e == nil {
		t.Fatal("duplicate file accepted")
	}
	req := ReceiptRequest{Summary: "第一版执行完成记录", FileIds: []string{file.Id}}
	wins.Store(0)
	for n := 0; n < 6; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, e := SubmitReceipt(temple, s, "TEST-TEMPLE", req); e == nil {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatal("duplicate receipt committed")
	}
	w = httptest.NewRecorder()
	ReceiptDownload(w, httptest.NewRequest("GET", "/", nil).WithContext(user), s, "TEST-TEMPLE", file.Id)
	if w.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("receipt inaccessible %s", w.Body.String())
	}
	w = httptest.NewRecorder()
	ReceiptDownload(w, httptest.NewRequest("GET", "/", nil).WithContext(intruder), s, "TEST-TEMPLE", file.Id)
	if w.Header().Get("Content-Type") == "image/png" {
		t.Fatal("cross user download")
	}
	if _, e := ReceiptDecision(temple, s, "TEST-TEMPLE", "", true); e == nil {
		t.Fatal("provider self accepted")
	}
	if _, e := ReceiptDecision(user, s, "TEST-TEMPLE", "请补充服务执行全景图片", false); e != nil {
		t.Fatal(e)
	}
	f2 := upload("TEST-TEMPLE", temple)
	if _, e := SubmitReceipt(temple, s, "TEST-TEMPLE", ReceiptRequest{Summary: "补充全景图片及执行说明", FileIds: []string{f2.Id}}); e != nil {
		t.Fatal(e)
	}
	if _, e := ReceiptDecision(user, s, "TEST-TEMPLE", "", true); e != nil {
		t.Fatal(e)
	}
	if _, e := ReceiptDecision(user, s, "TEST-TEMPLE", "", true); e == nil {
		t.Fatal("double completion")
	}
	progress, e := Fulfillment(user, s, "TEST-TEMPLE")
	if e != nil {
		t.Fatal(e)
	}
	if progress["status"] != "completed" || len(progress["receipts"].([]Receipt)) != 2 {
		t.Fatal("lost receipt versions")
	}
	if len(progress["logs"].([]*model.BookingStatusLog)) != 6 {
		t.Fatal("audit incomplete")
	}
	if e := os.WriteFile(filepath.Join(receiptDir(), file.Id), []byte("tampered"), 0600); e != nil {
		t.Fatal(e)
	}
	w = httptest.NewRecorder()
	ReceiptDownload(w, httptest.NewRequest("GET", "/", nil).WithContext(user), s, "TEST-TEMPLE", file.Id)
	if !strings.Contains(w.Body.String(), "50034") {
		t.Fatal("tamper not detected")
	}
	// Audit failure must roll back status as well.
	if _, e := db.ExecCtx(ctx, "RENAME TABLE booking_status_log TO unavailable_log"); e != nil {
		t.Fatal(e)
	}
	e = TransitionWithAudit(ctx, s, "TEST-MASTER", model.StatusConfirmed, "6", "master", "")
	_, _ = db.ExecCtx(ctx, "RENAME TABLE unavailable_log TO booking_status_log")
	if e == nil {
		t.Fatal("missing audit did not fail")
	}
	b, _ := s.BookingModel.FindOne(ctx, "TEST-MASTER")
	if b.Status != "pending" {
		t.Fatal("partial commit after audit failure")
	}
	// The same receipt requirement applies to independently booked masters.
	if _, e := NewMasterBookingConfirmLogic(master, s).MasterBookingConfirm(&types.AdminBookingActionReq{Id: "TEST-MASTER"}); e != nil {
		t.Fatal(e)
	}
	if _, e := NewMasterBookingStartLogic(master, s).MasterBookingStart(&types.AdminBookingActionReq{Id: "TEST-MASTER"}); e != nil {
		t.Fatal(e)
	}
	masterFile := upload("TEST-MASTER", master)
	if _, e := SubmitReceipt(master, s, "TEST-MASTER", ReceiptRequest{Summary: "独立大师执行回执说明", FileIds: []string{masterFile.Id}}); e != nil {
		t.Fatal(e)
	}
	if _, e := ReceiptDecision(user, s, "TEST-MASTER", "", true); e != nil {
		t.Fatal(e)
	}
	var count int
	if e := db.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM event_outbox WHERE aggregate_id='TEST-TEMPLE'"); e != nil || count != 6 {
		t.Fatalf("durable notification count: %d %v", count, e)
	}

}
