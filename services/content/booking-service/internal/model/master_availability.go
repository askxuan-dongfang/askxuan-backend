package model

import (
	"context"
	"github.com/askxuan/common/scheduling"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func checkMasterOverlap(ctx context.Context, q sqlx.Session, master, date, slot string) error {
	a, b, ok := scheduling.Range(slot)
	if !ok {
		return scheduling.ErrUnavailable
	}
	var n int64
	err := q.QueryRowCtx(ctx, &n, `SELECT COUNT(*) FROM booking WHERE master_code=? AND booking_date=? AND status NOT IN ('cancelled','completed','reviewed') AND (payment_status='success' OR (status='pending_payment' AND payment_expire_time>NOW())) AND SUBSTRING_INDEX(time_slot,'-',1)<? AND SUBSTRING_INDEX(time_slot,'-',-1)>?`, master, date, b, a)
	if err != nil {
		return err
	}
	if n > 0 {
		return scheduling.ErrUnavailable
	}
	return nil
}
func MasterSlotAvailable(ctx context.Context, db sqlx.SqlConn, master, date, slot string) (bool, error) {
	if err := scheduling.Check(ctx, db, master, date, slot); err != nil {
		if err == scheduling.ErrUnavailable {
			return false, nil
		}
		return false, err
	}
	if err := checkMasterOverlap(ctx, db, master, date, slot); err != nil {
		if err == scheduling.ErrUnavailable {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
