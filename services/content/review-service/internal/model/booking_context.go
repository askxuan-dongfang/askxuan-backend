package model

import (
	"context"
	"github.com/askxuan/review-service/internal/mq"
)

type templeScopeKey struct{}

func WithTemple(ctx context.Context, code string) context.Context {
	return context.WithValue(ctx, templeScopeKey{}, code)
}
func SyncBookingReview(ctx context.Context, e mq.BookingReviewed) error {
	if err := UpsertBookingReview(ctx, e.BookingId, e.UserId, e.MasterId, e.Rating, e.ReviewContent, e.ReviewImages); err != nil {
		return err
	}
	_, err := db.ExecCtx(ctx, `INSERT INTO review_booking_context(booking_id,temple_code,temple_name,master_name,service_name,master_reply) VALUES(?,?,?,?,?,?) ON DUPLICATE KEY UPDATE temple_code=VALUES(temple_code),temple_name=VALUES(temple_name),master_name=VALUES(master_name),service_name=VALUES(service_name),master_reply=IF(VALUES(master_reply)='',master_reply,VALUES(master_reply))`, e.BookingId, e.TempleId, e.TempleName, e.MasterName, e.ServiceName, e.MasterReply)
	if err != nil {
		return err
	}
	if e.Time != "" {
		_, err = db.ExecCtx(ctx, "UPDATE review SET create_time=? WHERE target_type='booking' AND target_id=?", e.Time, e.BookingId)
	}
	return err
}
