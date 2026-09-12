package logic

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"encoding/json"
	"github.com/askxuan/booking-service/internal/model"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/booking-service/internal/types"
	masterrpc "github.com/askxuan/booking-service/rpc/master"
	commonim "github.com/askxuan/common/im"
	"github.com/askxuan/common/middleware"
	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type chatTestBookings struct{ model.BookingModel }

func (chatTestBookings) FindOne(ctx context.Context, id string) (*model.Booking, error) {
	return &model.Booking{Id: id, UserId: "98765001", MasterId: "CHATTEST", MasterName: "测试师傅", PaymentStatus: model.PaymentStatusSuccess, Status: model.StatusPending}, nil
}

type chatTestMasters struct{}

func (chatTestMasters) GetByID(ctx context.Context, id int64) (*masterrpc.BookingMaster, error) {
	code := "CHATTEST"
	if id != 98765002 {
		code = "OTHER"
	}
	return &masterrpc.BookingMaster{Id: id, Code: code}, nil
}
func (chatTestMasters) GetByCode(ctx context.Context, code string) (*masterrpc.BookingMaster, error) {
	return &masterrpc.BookingMaster{Id: 98765002, Code: code}, nil
}

type chatTestIM struct {
	fail  bool
	count int
}

func (m *chatTestIM) SendMessage(context.Context, *commonim.SendMsgReq) error {
	m.count++
	if m.fail {
		return fmt.Errorf("isolated simulated outage")
	}
	return nil
}
func chatTestContext(master bool) context.Context {
	ctx := context.WithValue(context.Background(), middleware.CtxKeyUserID, int64(98765001))
	if master {
		ctx = context.WithValue(ctx, middleware.CtxKeyMasterID, int64(98765002))
	}
	return ctx
}

// Runs only against an explicitly named disposable schema; never sends to OpenIM.
func TestChatIntegration(t *testing.T) {
	dsn := os.Getenv("CHAT_TEST_DSN")
	if dsn == "" {
		t.Skip("set CHAT_TEST_DSN to a disposable askxuan_chat_test_* schema")
	}
	db := sqlx.NewMysql(dsn)
	ctx := context.Background()
	var database string
	if err := db.QueryRowCtx(ctx, &database, "SELECT DATABASE()"); err != nil {
		t.Fatal("test database unavailable")
	}
	if !strings.HasPrefix(database, "askxuan_chat_test_") {
		t.Fatal("refusing non-test database")
	}
	ddl := []string{
		`CREATE TABLE booking_chat_message(id BIGINT AUTO_INCREMENT PRIMARY KEY,booking_id VARCHAR(64),source_type VARCHAR(16),client_message_id VARCHAR(128),openim_server_msg_id VARCHAR(128) NOT NULL DEFAULT '',sender_type VARCHAR(16),sender_id VARCHAR(64),receiver_id VARCHAR(64),content VARCHAR(2000),kind VARCHAR(16) NOT NULL DEFAULT 'text',attachment_json TEXT,status VARCHAR(16),notify_status VARCHAR(16) NOT NULL DEFAULT 'done',notify_attempts INT NOT NULL DEFAULT 0,notify_after DATETIME,push_status VARCHAR(16) NOT NULL DEFAULT 'done',push_after DATETIME,create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,UNIQUE KEY dedup(booking_id,client_message_id)) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE chat_read_cursor(conversation_id VARCHAR(64),reader_id VARCHAR(64),through_id BIGINT NOT NULL DEFAULT 0,updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,PRIMARY KEY(conversation_id,reader_id)) ENGINE=InnoDB`,
		`CREATE TABLE chat_attachment(id VARCHAR(36) PRIMARY KEY,conversation_id VARCHAR(64),owner_id VARCHAR(64),name VARCHAR(255),content_type VARCHAR(128),file_size BIGINT,duration DOUBLE,created_at DATETIME DEFAULT CURRENT_TIMESTAMP) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE chat_call(id VARCHAR(36) PRIMARY KEY,conversation_id VARCHAR(64),caller_id VARCHAR(64),callee_id VARCHAR(64),kind VARCHAR(16),state VARCHAR(16) DEFAULT 'ringing',active_key VARCHAR(64),offer MEDIUMTEXT,answer MEDIUMTEXT,created_at DATETIME DEFAULT CURRENT_TIMESTAMP,updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,caller_seen DATETIME DEFAULT CURRENT_TIMESTAMP,callee_seen DATETIME DEFAULT CURRENT_TIMESTAMP,ended_at DATETIME,UNIQUE KEY active(active_key))`,
		`CREATE TABLE chat_call_signal(id BIGINT AUTO_INCREMENT PRIMARY KEY,call_id VARCHAR(36),sender_id VARCHAR(64),client_id VARCHAR(64),candidate TEXT,created_at DATETIME DEFAULT CURRENT_TIMESTAMP,UNIQUE KEY dedup(call_id,sender_id,client_id))`,
	}
	for _, q := range ddl {
		if _, err := db.ExecCtx(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, table := range []string{"booking_chat_message", "chat_read_cursor", "chat_attachment", "chat_call", "chat_call_signal"} {
			_, _ = db.ExecCtx(ctx, "DROP TABLE IF EXISTS "+table)
		}
	})
	fake := &chatTestIM{fail: true}
	s := &svc.ServiceContext{DB: db, ChatModel: model.NewBookingChatModel(db), BookingModel: chatTestBookings{}, MasterClient: chatTestMasters{}, IMClient: fake}
	customer, master := chatTestContext(false), chatTestContext(true)
	t.Run("concurrent retry accepts exactly once during IM outage", func(t *testing.T) {
		var wg sync.WaitGroup
		errs := make(chan error, 16)
		ids := make(chan int64, 16)
		for i := 0; i < 16; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				m, err := NewChatMessageSendLogic(customer, s).Send(&types.ChatMessageSendReq{Id: "TEST-A", ClientMessageId: "same-stable-id", Content: "原始消息"})
				if err != nil {
					errs <- err
				} else {
					ids <- m.Id
				}
			}()
		}
		wg.Wait()
		close(errs)
		close(ids)
		for err := range errs {
			t.Error(err)
		}
		var id int64
		for got := range ids {
			if id == 0 {
				id = got
			}
			if got != id {
				t.Error("duplicate business rows")
			}
		}
		if fake.count != 0 {
			t.Fatal("request path must not synchronously call OpenIM")
		}
		retry, err := NewChatMessageSendLogic(customer, s).Send(&types.ChatMessageSendReq{Id: "TEST-A", ClientMessageId: "same-stable-id", Content: "不同内容"})
		if err != nil || retry.Content != "原始消息" {
			t.Fatal("retry replaced original payload")
		}
		if _, err = NewChatMessageSendLogic(master, s).Send(&types.ChatMessageSendReq{Id: "TEST-A", ClientMessageId: "same-stable-id", Content: "伪造"}); err == nil {
			t.Fatal("peer reused another sender's message id")
		}
		page, err := NewChatMessageListLogic(master, s).List(&types.ChatMessageListReq{Id: "TEST-A", Size: 50})
		if err != nil || len(page.List) != 1 {
			t.Fatal("accepted message unavailable during IM outage")
		}
		drainChatOutbox(ctx, s)
		var state string
		_ = db.QueryRowCtx(ctx, &state, "SELECT notify_status FROM booking_chat_message WHERE id=?", id)
		if state != "pending" {
			t.Fatal("failed wakeup was not retained")
		}
		fake.fail = false
		_, _ = db.ExecCtx(ctx, "UPDATE booking_chat_message SET notify_after=NOW() WHERE id=?", id)
		drainChatOutbox(ctx, s)
		_ = db.QueryRowCtx(ctx, &state, "SELECT notify_status FROM booking_chat_message WHERE id=?", id)
		if state != "done" {
			t.Fatal("outbox failed to recover")
		}
	})
	t.Run("stable history and monotonic per-reader cursors", func(t *testing.T) {
		for i := 0; i < 125; i++ {
			_, err := NewChatMessageSendLogic(customer, s).Send(&types.ChatMessageSendReq{Id: "TEST-HISTORY", ClientMessageId: fmt.Sprint(i), Content: fmt.Sprintf("消息 %d", i)})
			if err != nil {
				t.Fatal(err)
			}
		}
		newest, err := NewChatMessageListLogic(master, s).List(&types.ChatMessageListReq{Id: "TEST-HISTORY", Size: 50})
		if err != nil || !newest.HasMore || len(newest.List) != 50 {
			t.Fatal("first window incorrect")
		}
		unread, _, _, err := chatReadState(master, s, "TEST-HISTORY", "m_98765002", "u_98765001")
		if err != nil || unread != 125 {
			t.Fatalf("unread=%d %v", unread, err)
		}
		_, err = NewChatMessageSendLogic(customer, s).Send(&types.ChatMessageSendReq{Id: "TEST-HISTORY", ClientMessageId: "insert-between-pages", Content: "新消息"})
		if err != nil {
			t.Fatal(err)
		}
		older, err := NewChatMessageListLogic(master, s).List(&types.ChatMessageListReq{Id: "TEST-HISTORY", BeforeId: newest.List[0].Id, Size: 50})
		if err != nil || len(older.List) != 50 || older.List[len(older.List)-1].Id >= newest.List[0].Id {
			t.Fatal("history cursor overlaps")
		}
		last := newest.List[len(newest.List)-1].Id
		if _, err = ChatMarkRead(master, s, &types.ChatReadReq{Id: "TEST-HISTORY", ThroughId: last}); err != nil {
			t.Fatal(err)
		}
		_, _ = ChatMarkRead(master, s, &types.ChatReadReq{Id: "TEST-HISTORY", ThroughId: older.List[0].Id})
		unread, read, _, _ := chatReadState(master, s, "TEST-HISTORY", "m_98765002", "u_98765001")
		if unread != 1 || read != last {
			t.Fatalf("cursor regressed read=%d unread=%d", read, unread)
		}
		after, err := NewChatMessageListLogic(master, s).List(&types.ChatMessageListReq{Id: "TEST-HISTORY", AfterId: last, Size: 50})
		if err != nil || len(after.List) != 1 {
			t.Fatal("incremental sync incorrect")
		}
		_, _, peerRead, _ := chatReadState(customer, s, "TEST-HISTORY", "u_98765001", "m_98765002")
		if peerRead != last {
			t.Fatal("read receipt not visible to sender")
		}
		unauthorized := context.WithValue(context.Background(), middleware.CtxKeyUserID, int64(123))
		if _, err = ChatMarkRead(unauthorized, s, &types.ChatReadReq{Id: "TEST-HISTORY", ThroughId: last}); err == nil {
			t.Fatal("non-participant marked read")
		}
	})
	t.Run("call lifecycle enforces participants and expires stale sessions", func(t *testing.T) {
		t.Setenv("CHAT_TURN_SECRET", "isolated-test-secret")
		t.Setenv("CHAT_TURN_URLS", "turn:127.0.0.1:3478")
		id := uuid.NewString()
		req := &ChatCallReq{Id: "TEST-CALL", Action: "start", Kind: "video", SDP: "v=0\r\nisolated-offer", ClientID: id}
		if _, err := ChatCallAction(customer, s, req); err != nil {
			t.Fatal(err)
		}
		if _, err := ChatCallAction(customer, s, req); err != nil {
			t.Fatal("call retry was not idempotent", err)
		}
		req.ClientID = uuid.NewString()
		if _, err := ChatCallAction(customer, s, req); err == nil {
			t.Fatal("overlapping conversation call accepted")
		}
		if _, err := ChatCallAction(customer, s, &ChatCallReq{Id: "TEST-CALL", Call: id, Action: "accept", SDP: "v=0\r\nanswer"}); err == nil {
			t.Fatal("caller accepted its own invitation")
		}
		if _, err := ChatCallAction(master, s, &ChatCallReq{Id: "TEST-CALL", Call: id, Action: "accept", SDP: "v=0\r\nanswer"}); err != nil {
			t.Fatal(err)
		}
		candidate := &ChatCallReq{Id: "TEST-CALL", Call: id, Action: "candidate", ClientID: "stable", Candidate: `{"candidate":"test-only"}`}
		for i := 0; i < 2; i++ {
			if _, err := ChatCallAction(customer, s, candidate); err != nil {
				t.Fatal(err)
			}
		}
		result, err := ChatCallGet(master, s, &ChatCallReq{Id: "TEST-CALL", Call: id})
		if err != nil || len(result["signals"].([]CallSignal)) != 1 {
			t.Fatal("candidate deduplication failed")
		}
		intruder := context.WithValue(context.Background(), middleware.CtxKeyUserID, int64(123))
		if _, err := ChatCallGet(intruder, s, &ChatCallReq{Id: "TEST-CALL", Call: id}); err == nil {
			t.Fatal("signaling leaked to non-participant")
		}
		_, _ = db.ExecCtx(ctx, "UPDATE chat_call SET caller_seen=NOW()-INTERVAL 1 MINUTE WHERE id=?", id)
		result, err = ChatCallGet(master, s, &ChatCallReq{Id: "TEST-CALL", Call: id})
		if err != nil || result["call"].(ChatCall).State != "ended" {
			t.Fatal("stale session retained device reservation")
		}
		if _, err := ChatCallAction(master, s, &ChatCallReq{Id: "TEST-CALL", Call: id, Action: "accept", SDP: "v=0\r\nanswer"}); err == nil {
			t.Fatal("ended call reopened")
		}
	})
	t.Run("private attachments authorize upload send and download", func(t *testing.T) {
		t.Setenv("CHAT_FILES_DIR", t.TempDir())
		var body bytes.Buffer
		multi := multipart.NewWriter(&body)
		part, _ := multi.CreateFormFile("file", "测试.png")
		_ = png.Encode(part, image.NewRGBA(image.Rect(0, 0, 4, 4)))
		multi.Close()
		request := httptest.NewRequest("POST", "/upload", &body).WithContext(customer)
		request.Header.Set("Content-Type", multi.FormDataContentType())
		response := httptest.NewRecorder()
		ChatUpload(response, request, s, "TEST-FILE")
		var envelope struct {
			Code int                  `json:"code"`
			Data types.ChatAttachment `json:"data"`
		}
		if json.Unmarshal(response.Body.Bytes(), &envelope) != nil || envelope.Code != 0 || envelope.Data.Id == "" {
			t.Fatalf("upload failed: %s", response.Body.String())
		}
		a := envelope.Data
		before := httptest.NewRecorder()
		ChatDownload(before, httptest.NewRequest("GET", "/file", nil).WithContext(master), s, "TEST-FILE", a.Id)
		if bytes.Equal(before.Body.Bytes(), body.Bytes()) || strings.HasPrefix(before.Header().Get("Content-Type"), "image/") {
			t.Fatal("unsent attachment leaked")
		}
		if _, err := NewChatMessageSendLogic(customer, s).Send(&types.ChatMessageSendReq{Id: "OTHER", ClientMessageId: "wrong-conversation", Kind: "image", AttachmentId: a.Id}); err == nil {
			t.Fatal("cross-conversation attachment accepted")
		}
		if _, err := NewChatMessageSendLogic(customer, s).Send(&types.ChatMessageSendReq{Id: "TEST-FILE", ClientMessageId: "image", Kind: "image", AttachmentId: a.Id}); err != nil {
			t.Fatal(err)
		}
		received := httptest.NewRecorder()
		ChatDownload(received, httptest.NewRequest("GET", "/file", nil).WithContext(master), s, "TEST-FILE", a.Id)
		if received.Code != 200 || received.Header().Get("Content-Type") != "image/png" || !bytes.HasPrefix(received.Body.Bytes(), []byte{137, 80, 78, 71}) {
			t.Fatal("private image not delivered")
		}
		intruder := context.WithValue(context.Background(), middleware.CtxKeyUserID, int64(123))
		denied := httptest.NewRecorder()
		ChatDownload(denied, httptest.NewRequest("GET", "/file", nil).WithContext(intruder), s, "TEST-FILE", a.Id)
		if bytes.HasPrefix(denied.Body.Bytes(), []byte{137, 80, 78, 71}) {
			t.Fatal("attachment bytes leaked")
		}
	})
}
func TestChatContentTypes(t *testing.T) {
	for _, mime := range []string{"image/svg+xml", "text/html", "application/javascript"} {
		if _, err := chatContentType(mime, mime, "attack.html", []byte("<html>")); err == nil {
			t.Errorf("active content accepted: %s", mime)
		}
	}
	if got, err := chatContentType("video/webm", "audio/webm", "voice.webm", nil); err != nil || got != "audio/webm" {
		t.Fatal("recorded audio container rejected")
	}
}
