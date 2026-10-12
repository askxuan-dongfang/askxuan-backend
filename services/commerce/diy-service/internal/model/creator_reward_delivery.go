package model

import (
	"context"
	"fmt"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Cross-schema writes share the same InnoDB transaction as reward settlement.
func grantNonCashReward(ctx context.Context, tx sqlx.Session, r CreatorReward, reverse bool) error {
	key := fmt.Sprintf("diy-reward:%d", r.ID)
	if r.Kind == "balance" {
		if _, e := tx.ExecCtx(ctx, "INSERT INTO askxuan_payment.wallet_account(user_id) VALUES(?) ON DUPLICATE KEY UPDATE user_id=user_id", r.Creator); e != nil {
			return e
		}
		var a struct {
			Available int64 `db:"available_cents"`
			Held      int64 `db:"held_cents"`
		}
		if e := tx.QueryRowCtx(ctx, &a, "SELECT available_cents,held_cents FROM askxuan_payment.wallet_account WHERE user_id=? FOR UPDATE", r.Creator); e != nil {
			return e
		}
		delta := r.Amount
		kind := "creator_reward"
		if reverse {
			key += "-reverse"
			kind = "creator_reward_reversal"
			deduct := r.Amount
			if a.Available < deduct {
				deduct = a.Available
			}
			delta = -deduct
			if debt := r.Amount - deduct; debt > 0 {
				if e := cashChange(ctx, tx, r.Creator, key, "refund_debt", r.OrderNo, -debt, 0, 0); e != nil {
					return e
				}
			}
		}
		if _, e := tx.ExecCtx(ctx, "UPDATE askxuan_payment.wallet_account SET available_cents=available_cents+? WHERE user_id=?", delta, r.Creator); e != nil {
			return e
		}
		_, e := tx.ExecCtx(ctx, "INSERT INTO askxuan_payment.wallet_ledger(user_id,event_key,kind,reference_no,available_delta,held_delta,available_after,held_after) VALUES(?,?,?,?,?,0,?,?)", r.Creator, key, kind, r.OrderNo, delta, a.Available+delta, a.Held)
		return e
	}
	if r.Kind != "coupon" {
		return common.ErrParam
	}
	if reverse {
		var record int64
		if e := tx.QueryRowCtx(ctx, &record, "SELECT coupon_record_id FROM creator_reward_delivery WHERE reward_id=?", r.ID); e != nil {
			return e
		}
		var status string
		if e := tx.QueryRowCtx(ctx, &status, "SELECT status FROM askxuan_marketing.coupon_record WHERE id=? FOR UPDATE", record); e != nil {
			return e
		}
		if status == "unused" {
			_, e := tx.ExecCtx(ctx, "UPDATE askxuan_marketing.coupon_record SET status='expired' WHERE id=?", record)
			return e
		}
		// Consumed coupons require an operator's reconciliation, not a guessed cash debt.
		if status == "used" {
			return rewardAudit(ctx, tx, "system", "coupon_refund_review", r.OrderNo, fmt.Sprintf("奖励优惠券 %d 已使用，需人工核对关联消费订单", record))
		}
		return nil
	}
	var template struct {
		Count    int `db:"total_count"`
		Received int `db:"received_count"`
		Valid    int `db:"valid"`
	}
	if e := tx.QueryRowCtx(ctx, &template, "SELECT total_count,received_count,(status='enabled' AND (start_time IS NULL OR start_time<=NOW()) AND (end_time IS NULL OR end_time>NOW())) valid FROM askxuan_marketing.coupon WHERE id=? FOR UPDATE", r.CouponID); e != nil {
		return e
	}
	if template.Valid != 1 || template.Received >= template.Count {
		return common.NewBizError(40901, "奖励优惠券已失效或库存不足，请管理台补充后重试结算")
	}
	no := fmt.Sprintf("DCR%d", r.ID)
	result, e := tx.ExecCtx(ctx, `INSERT INTO askxuan_marketing.coupon(coupon_no,name,type,value,min_amount,category_id,start_time,end_time,total_count,received_count,status) SELECT ?,name,type,value,min_amount,category_id,start_time,end_time,1,1,'enabled' FROM askxuan_marketing.coupon WHERE id=?`, no, r.CouponID)
	if e != nil {
		return e
	}
	coupon, e := result.LastInsertId()
	if e != nil {
		return e
	}
	result, e = tx.ExecCtx(ctx, "INSERT INTO askxuan_marketing.coupon_record(coupon_id,coupon_no,user_id,status) VALUES(?,?,?,'unused')", coupon, no, r.Creator)
	if e != nil {
		return e
	}
	record, e := result.LastInsertId()
	if e != nil {
		return e
	}
	if _, e = tx.ExecCtx(ctx, "UPDATE askxuan_marketing.coupon SET received_count=received_count+1 WHERE id=?", r.CouponID); e != nil {
		return e
	}
	_, e = tx.ExecCtx(ctx, "INSERT INTO creator_reward_delivery(reward_id,coupon_record_id) VALUES(?,?)", r.ID, record)
	return e
}
