package model

import "context"

type RatingSummary struct {
	Code    string  `db:"code" json:"code"`
	Kind    string  `db:"kind" json:"kind"`
	Count   int64   `db:"count" json:"count"`
	Average float64 `db:"average" json:"average"`
}

func Ratings(ctx context.Context) ([]RatingSummary, error) {
	rows := []RatingSummary{}
	err := db.QueryRowsCtx(ctx, &rows, `SELECT master_code code,'master' kind,COUNT(*) count,ROUND(AVG(rating),1) average FROM review JOIN review_booking_context c ON review.target_id=c.booking_id WHERE status='normal' AND target_type='booking' AND master_code<>'' GROUP BY master_code UNION ALL SELECT c.temple_code code,'temple' kind,COUNT(*) count,ROUND(AVG(r.rating),1) average FROM review r JOIN review_booking_context c ON r.target_type='booking' AND r.target_id=c.booking_id WHERE r.status='normal' AND c.temple_code<>'' GROUP BY c.temple_code`)
	return rows, err
}
