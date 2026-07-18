package model

import (
	"context"
	"errors"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	CouponRecordUnused  = "unused"
	CouponRecordUsed    = "used"
	CouponRecordExpired = "expired"
)

type CouponRecord struct {
	Id        int64  `db:"id" json:"id"`
	CouponId  int64  `db:"coupon_id" json:"couponId"`
	CouponNo  string `db:"coupon_no" json:"couponNo"`
	UserId    string `db:"user_id" json:"userId"`
	Status    string `db:"status" json:"status"`
	OrderNo   string `db:"order_no" json:"orderNo"`
	UseTime   string `db:"use_time" json:"useTime"`
	CreatedAt string `db:"created_at" json:"createdAt"`
}

const recordColumns = `id,coupon_id,coupon_no,user_id,status,order_no,IFNULL(DATE_FORMAT(use_time,'%Y-%m-%d %H:%i:%s'),'') use_time,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') created_at`

func ReceiveCoupon(couponId int64, userId string) (CouponRecord, error) {
	if userId == "" {
		return CouponRecord{}, fmt.Errorf("用户不能为空")
	}
	ctx := context.Background()
	var out CouponRecord
	err := db.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		var existing CouponRecord
		err := session.QueryRowCtx(ctx, &existing, `SELECT `+recordColumns+` FROM coupon_record WHERE coupon_id=? AND user_id=? LIMIT 1 FOR UPDATE`, couponId, userId)
		if err == nil {
			out = existing
			return nil
		}
		if !errors.Is(err, sqlx.ErrNotFound) {
			return err
		}
		coupon, err := findCouponForUpdate(ctx, session, couponId)
		if err != nil {
			return fmt.Errorf("优惠券不存在")
		}
		if coupon.Status != StatusEnabled {
			return fmt.Errorf("优惠券已下架")
		}
		if coupon.ReceivedCount >= coupon.TotalCount {
			return fmt.Errorf("优惠券已领完")
		}
		if _, err = session.ExecCtx(ctx, `UPDATE coupon SET received_count=received_count+1 WHERE id=? AND received_count<total_count`, couponId); err != nil {
			return err
		}
		res, err := session.ExecCtx(ctx, `INSERT INTO coupon_record(coupon_id,coupon_no,user_id,status) VALUES(?,?,?,?)`, couponId, coupon.CouponNo, userId, CouponRecordUnused)
		if err != nil {
			return err
		}
		out = CouponRecord{CouponId: couponId, CouponNo: coupon.CouponNo, UserId: userId, Status: CouponRecordUnused, CreatedAt: nowStr()}
		out.Id, _ = res.LastInsertId()
		return nil
	})
	return out, err
}

func ListMyCoupons(userId, status string, page, size int) ([]CouponRecord, int64) {
	where, args := "user_id=?", []interface{}{userId}
	if status != "" {
		where += " AND status=?"
		args = append(args, status)
	}
	var total int64
	if db.QueryRowCtx(context.Background(), &total, "SELECT COUNT(1) FROM coupon_record WHERE "+where, args...) != nil {
		return []CouponRecord{}, 0
	}
	offset, limit := pageArgs(page, size)
	var list []CouponRecord
	if db.QueryRowsCtx(context.Background(), &list, `SELECT `+recordColumns+` FROM coupon_record WHERE `+where+` ORDER BY id DESC LIMIT ?,?`, append(args, offset, limit)...) != nil {
		return []CouponRecord{}, 0
	}
	return list, total
}
