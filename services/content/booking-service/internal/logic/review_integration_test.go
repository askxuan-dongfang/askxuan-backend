package logic

import (
	"context"
	"github.com/askxuan/booking-service/internal/model"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/booking-service/internal/types"
	"github.com/askxuan/common/middleware"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestReviewTransactionIntegration(t *testing.T) {
	dsn := os.Getenv("REVIEW_BOOKING_TEST_DSN")
	if dsn == "" {
		t.Skip("requires disposable review_test_* DB")
	}
	db := sqlx.NewMysql(dsn)
	ctx := context.Background()
	sqlx.DisableLog()
	var name string
	if err := db.QueryRowCtx(ctx, &name, "SELECT DATABASE()"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(name, "review_test_") {
		t.Fatal("not test database")
	}
	init, err := os.ReadFile("../../../../../db/init.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"booking", "booking_review", "booking_status_log", "event_outbox"} {
		prefix := "CREATE TABLE IF NOT EXISTS `" + table + "`"
		if table == "booking" {
			prefix = "CREATE TABLE `booking`"
		}
		if table == "event_outbox" {
			prefix = "CREATE TABLE IF NOT EXISTS `askxuan_booking`.`event_outbox`"
		}
		i := strings.Index(string(init), prefix)
		if i < 0 {
			t.Fatal(table)
		}
		ddl := strings.ReplaceAll(strings.SplitN(string(init)[i:], ";", 2)[0], "`askxuan_booking`.", "")
		if _, err = db.ExecCtx(ctx, ddl); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = db.ExecCtx(ctx, "DROP TABLE IF EXISTS "+table) })
	}
	s := &svc.ServiceContext{DB: db, BookingModel: model.NewBookingModel(db), ReviewModel: model.NewBookingReviewModel(db)}
	user := context.WithValue(ctx, middleware.CtxKeyUserID, int64(1))
	user = context.WithValue(user, middleware.CtxKeyRoles, []string{"customer"})
	insert := func(id, status string) {
		_, err := db.ExecCtx(ctx, `INSERT INTO booking(booking_no,user_id,temple_code,temple_name,master_code,master_name,service_code,service_name,booking_date,time_slot,status,payment_status) VALUES(?,1,'TTEST','Test','','','S','Service',CURDATE(),'09:00',?,'success')`, id, status)
		if err != nil {
			t.Fatal(err)
		}
	}
	insert("BTEST", "completed")
	req := &types.ReviewCreateReq{Id: "BTEST", Rating: 4, Content: "实际体验", Images: []string{}}
	intruder := context.WithValue(user, middleware.CtxKeyUserID, int64(2))
	if _, err := NewCreateReviewLogic(intruder, s).CreateReview(req); err == nil {
		t.Fatal("cross user accepted")
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := NewCreateReviewLogic(user, s).CreateReview(req); err == nil {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("concurrent successes %d", successes.Load())
	}
	for _, table := range []string{"booking_review", "booking_status_log", "event_outbox"} {
		var count int
		if err := db.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM "+table); err != nil || count != 1 {
			t.Fatalf("%s count %d: %v", table, count, err)
		}
	}
	var payload string
	if err := db.QueryRowCtx(ctx, &payload, "SELECT CAST(payload AS CHAR) FROM event_outbox LIMIT 1"); err != nil || !strings.Contains(payload, "TTEST") {
		t.Fatal("missing temple metadata", err)
	}
	insert("BFAIL", "completed")
	if _, err := db.ExecCtx(ctx, "RENAME TABLE event_outbox TO event_outbox_unavailable"); err != nil {
		t.Fatal(err)
	}
	_, err = NewCreateReviewLogic(user, s).CreateReview(&types.ReviewCreateReq{Id: "BFAIL", Rating: 5, Content: "测试回滚"})
	_, restore := db.ExecCtx(ctx, "RENAME TABLE event_outbox_unavailable TO event_outbox")
	if restore != nil {
		t.Fatal(restore)
	}
	if err == nil {
		t.Fatal("outbox failure silently accepted")
	}
	var status string
	_ = db.QueryRowCtx(ctx, &status, "SELECT status FROM booking WHERE booking_no='BFAIL'")
	if status != "completed" {
		t.Fatal("state not rolled back")
	}
	var count int
	_ = db.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM booking_review WHERE booking_id='BFAIL'")
	if count != 0 {
		t.Fatal("review not rolled back")
	}
	insert("BPENDING", "pending")
	if _, err := NewCreateReviewLogic(user, s).CreateReview(&types.ReviewCreateReq{Id: "BPENDING", Rating: 5, Content: "提前评价"}); err == nil {
		t.Fatal("unfinished booking reviewed")
	}
}
