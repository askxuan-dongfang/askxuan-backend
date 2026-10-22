package logic

import (
	"context"
	"github.com/askxuan/booking-service/internal/model"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"testing"
)

func TestOpenIMBanAndReplayMySQL(t *testing.T) {
	dsn := os.Getenv("ACCOUNT_SYSTEM_TEST_DSN")
	if dsn == "" || os.Getenv("ACCOUNT_SYSTEM_TEST_ISOLATED") != "1" {
		t.Skip("isolated account fixture required")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil || cfg.DBName != "askxuan_account_test" {
		t.Fatal("refusing non-test database")
	}
	db := sqlx.NewMysql(dsn)
	ctx := context.Background()
	sqlx.DisableLog()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := db.ExecCtx(ctx, q, args...); e != nil {
			t.Fatal(e)
		}
	}
	exec(`CREATE TABLE IF NOT EXISTS booking_chat_message(id BIGINT AUTO_INCREMENT PRIMARY KEY,booking_id VARCHAR(64),source_type VARCHAR(16),client_message_id VARCHAR(128),openim_server_msg_id VARCHAR(128) NOT NULL DEFAULT '',sender_type VARCHAR(16),sender_id VARCHAR(64),receiver_id VARCHAR(64),content VARCHAR(2000),kind VARCHAR(16) NOT NULL DEFAULT 'text',attachment_json TEXT,status VARCHAR(16),notify_status VARCHAR(16),notify_after DATETIME,create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP)`)
	exec("DELETE FROM booking_chat_message WHERE booking_id='TEST-IMBAN'")
	exec("DELETE FROM askxuan_user.user WHERE id=98765001")
	exec("INSERT INTO askxuan_user.user(id,nickname,status) VALUES(98765001,'IM安全测试',1)")
	exec("INSERT INTO booking_chat_message(booking_id,source_type,client_message_id,sender_type,sender_id,receiver_id,content,kind,attachment_json,status,notify_status,notify_after) VALUES('TEST-IMBAN','booking','client-IMBAN','customer','u_98765001','m_98765002','原始内容','text','{}','sent','working',NOW()+INTERVAL 45 SECOND)")
	t.Cleanup(func() {
		db.ExecCtx(ctx, "DELETE FROM booking_chat_message WHERE booking_id='TEST-IMBAN'")
		db.ExecCtx(ctx, "DELETE FROM askxuan_user.user WHERE id=98765001")
	})
	s := &svc.ServiceContext{DB: db, BookingModel: chatTestBookings{}, ChatModel: model.NewBookingChatModel(db), MasterClient: chatTestMasters{}}
	check := func(content string, want bool) {
		t.Helper()
		_, e := AuthorizeOpenIMMessage(ctx, s, "u_98765001", "m_98765002", chatMessageMarker("booking", "TEST-IMBAN", "client-IMBAN"), content)
		if (e == nil) != want {
			t.Fatalf("allowed=%v err=%v", want, e)
		}
	}
	check(`{"content":"原始内容"}`, true)
	check(`{"content":"伪造内容"}`, false)
	exec("UPDATE askxuan_user.user SET status=0,authz_version=authz_version+1 WHERE id=98765001")
	check(`{"content":"原始内容"}`, false)
	exec("UPDATE askxuan_user.user SET status=1,authz_version=authz_version+1 WHERE id=98765001")
	exec("UPDATE booking_chat_message SET notify_status='done' WHERE booking_id='TEST-IMBAN'")
	check(`{"content":"原始内容"}`, false)
	if _, e := AuthorizeOpenIMMessage(ctx, s, "u_98765001", "u_1", "", `{"content":"旁路消息"}`); e == nil {
		t.Fatal("unsupported human chat bypass")
	}
}
