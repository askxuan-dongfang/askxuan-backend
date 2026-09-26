package model

import (
	"context"
	"github.com/askxuan/review-service/internal/mq"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"strings"
	"testing"
)

func TestProjectionIntegration(t *testing.T) {
	dsn := os.Getenv("REVIEW_TEST_DSN")
	if dsn == "" {
		t.Skip("requires disposable review_test_* DB")
	}
	db = sqlx.NewMysql(dsn)
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
	for _, table := range []string{"review", "review_booking_context"} {
		prefix := "CREATE TABLE IF NOT EXISTS `" + table + "`"
		if table == "review_booking_context" {
			prefix = "CREATE TABLE IF NOT EXISTS review_booking_context"
		}
		i := strings.Index(string(init), prefix)
		if i < 0 {
			t.Fatal(table)
		}
		if _, err = db.ExecCtx(ctx, strings.SplitN(string(init)[i:], ";", 2)[0]); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = db.ExecCtx(ctx, "DROP TABLE IF EXISTS "+table) })
	}
	e := mq.BookingReviewed{BookingId: "B1", UserId: "1", TempleId: "T1", TempleName: "Test", ServiceName: "Service", Rating: 5, ReviewContent: "full temple", ReviewImages: "[]", Time: "2026-09-20 12:00:00"}
	if err := SyncBookingReview(ctx, e); err != nil {
		t.Fatal(err)
	}
	e.BookingId = "B2"
	e.MasterId = "W003"
	e.Rating = 3
	if err := SyncBookingReview(ctx, e); err != nil {
		t.Fatal(err)
	}
	e.BookingId = "B3"
	e.Rating = 5
	if err := SyncBookingReview(ctx, e); err != nil {
		t.Fatal(err)
	}
	rows, total, err := ListReviews(WithTemple(ctx, "T1"), "", "", "", 0, ReviewStatusNormal, "", 1, 1)
	if err != nil || total != 3 || len(rows) != 1 || rows[0].ServiceName != "Service" {
		t.Fatalf("temple list %v %d %v", rows, total, err)
	}
	stats, err := Ratings(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range stats {
		if r.Kind == "master" && (r.Code != "W003" || r.Count != 2 || r.Average != 4) {
			t.Fatalf("stats %+v", stats)
		}
	}
	rows, _, _ = ListReviews(ctx, "booking", "B2", "", 0, "", "", 1, 20)
	_, err = UpdateReviewStatus(ctx, rows[0].Id, ReviewStatusHidden)
	if err != nil {
		t.Fatal(err)
	}
	e.BookingId = "B2"
	e.Rating = 3
	if err := SyncBookingReview(ctx, e); err != nil {
		t.Fatal(err)
	}
	rows, total, err = ListReviews(ctx, "", "", "", 0, ReviewStatusNormal, "W003", 1, 20)
	if err != nil || total != 1 || len(rows) != 1 {
		t.Fatal("hidden replay resurfaced", total, err)
	}
	rows, total, err = ListReviews(WithTemple(ctx, "OTHER"), "", "", "", 0, "", "", 1, 20)
	if err != nil || total != 0 {
		t.Fatal("cross temple leak")
	}
}
