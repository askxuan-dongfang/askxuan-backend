package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"math"
	"strings"
	"time"
)

type RewardPolicy struct {
	Enabled     bool   `db:"enabled" json:"enabled"`
	Kind        string `db:"kind" json:"kind"`
	RateBps     int64  `db:"rate_bps" json:"rateBps"`
	Cap         int64  `db:"cap_cents" json:"capCents"`
	WaitDays    int    `db:"wait_days" json:"waitDays"`
	MinWithdraw int64  `db:"min_withdraw_cents" json:"minWithdrawCents"`
	CouponID    int64  `db:"coupon_id" json:"couponId"`
	Revision    int    `db:"revision" json:"revision"`
}

const policyFields = "enabled,kind,rate_bps,cap_cents,wait_days,min_withdraw_cents,coupon_id,revision"

func ReadRewardPolicy(ctx context.Context, db sqlx.Session) (p RewardPolicy, err error) {
	err = db.QueryRowCtx(ctx, &p, "SELECT "+policyFields+" FROM creator_reward_policy WHERE id=1")
	return
}
func (p RewardPolicy) Validate() error {
	if (p.Kind != "cash" && p.Kind != "balance" && p.Kind != "coupon") || p.RateBps < 0 || p.RateBps > 10000 || p.Cap < 1 || p.Cap > 1000000 || p.WaitDays < 7 || p.WaitDays > 365 || p.MinWithdraw < 1 || p.MinWithdraw > 1000000 || (p.Kind == "coupon" && p.CouponID < 1) {
		return common.ErrParam
	}
	return nil
}
func RewardAmount(base, rate, cap int64) int64 {
	if base <= 0 || base > 100000000 || rate <= 0 || rate > 10000 || cap <= 0 {
		return 0
	}
	n := (base*rate + 5000) / 10000
	if n > cap {
		n = cap
	}
	return n
}
func SaveRewardPolicy(ctx context.Context, db sqlx.SqlConn, p RewardPolicy, operator string) error {
	if e := p.Validate(); e != nil {
		return e
	}
	return db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		result, e := tx.ExecCtx(ctx, "UPDATE creator_reward_policy SET enabled=?,kind=?,rate_bps=?,cap_cents=?,wait_days=?,min_withdraw_cents=?,coupon_id=?,revision=revision+1,updated_by=? WHERE id=1 AND revision=?", p.Enabled, p.Kind, p.RateBps, p.Cap, p.WaitDays, p.MinWithdraw, p.CouponID, operator, p.Revision)
		if e != nil {
			return e
		}
		n, _ := result.RowsAffected()
		if n != 1 {
			return common.NewBizError(40901, "规则已更新，请刷新后再保存")
		}
		b, _ := json.Marshal(p)
		return rewardAudit(ctx, tx, operator, "policy", "1", string(b))
	})
}
func rewardAudit(ctx context.Context, tx sqlx.Session, user, action, ref, detail string) error {
	_, e := tx.ExecCtx(ctx, "INSERT INTO creator_reward_audit(operator_id,action,reference_no,detail) VALUES(?,?,?,?)", user, action, ref, detail)
	return e
}

// Snapshot only future, attributed orders. Payment callbacks cannot change rules.
func SnapshotReward(ctx context.Context, tx sqlx.Session, o *DiyOrder) error {
	if o.CreatorId == "" || o.CreatorId == o.UserId {
		return nil
	}
	p, e := ReadRewardPolicy(ctx, tx)
	if e != nil {
		return e
	}
	if !p.Enabled {
		return nil
	}
	designID := o.DesignId
	var design DiyDesign
	if json.Unmarshal([]byte(o.DesignSnapshot), &design) == nil && design.SourceDesignId > 0 {
		seen := map[int64]bool{}
		for depth := 0; depth < 32; depth++ {
			next := design.SourceDesignId
			if next == 0 {
				break
			}
			if seen[next] {
				return common.ErrParam
			}
			seen[next] = true
			var source struct {
				ID     int64 `db:"id"`
				Source int64 `db:"source_design_id"`
			}
			if e := tx.QueryRowCtx(ctx, &source, "SELECT id,source_design_id FROM diy_design WHERE id=?", next); e != nil {
				return e
			}
			design.SourceDesignId = source.Source
			designID = source.ID
		}
		if design.SourceDesignId != 0 {
			return common.ErrParam
		}
	}
	base := int64(math.Round(o.MaterialFee * 100))
	amount := RewardAmount(base, p.RateBps, p.Cap)
	if amount <= 0 && p.Kind != "coupon" {
		return nil
	}
	_, e = tx.ExecCtx(ctx, `INSERT INTO creator_reward(order_id,order_no,design_id,creator_id,kind,rate_bps,cap_cents,wait_days,coupon_id,policy_revision,base_cents,amount_cents) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, o.Id, o.OrderNo, designID, o.CreatorId, p.Kind, p.RateBps, p.Cap, p.WaitDays, p.CouponID, p.Revision, base, amount)
	return e
}

type CreatorReward struct {
	ID       int64  `db:"id" json:"id"`
	OrderID  int64  `db:"order_id" json:"-"`
	OrderNo  string `db:"order_no" json:"orderNo"`
	DesignID int64  `db:"design_id" json:"designId"`
	Creator  string `db:"creator_id" json:"creatorId"`
	Kind     string `db:"kind" json:"kind"`
	Base     int64  `db:"base_cents" json:"baseCents"`
	Amount   int64  `db:"amount_cents" json:"amountCents"`
	Status   string `db:"status" json:"status"`
	Reason   string `db:"reason" json:"reason"`
	WaitDays int    `db:"wait_days" json:"waitDays"`
	CouponID int64  `db:"coupon_id" json:"couponId"`
	Created  string `db:"created_at" json:"createdAt"`
}

const rewardFields = "id,order_id,order_no,design_id,creator_id,kind,base_cents,amount_cents,status,reason,wait_days,coupon_id,DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s') created_at"

type CreatorAccount struct {
	Available int64 `db:"available_cents" json:"availableCents"`
	Held      int64 `db:"held_cents" json:"heldCents"`
	Paid      int64 `db:"paid_cents" json:"paidCents"`
}

// Caller locks account first for every withdrawal; reward operations also serialize here.
func lockCreator(ctx context.Context, tx sqlx.Session, user string) (a CreatorAccount, e error) {
	if _, e = tx.ExecCtx(ctx, "INSERT INTO creator_cash_account(user_id) VALUES(?) ON DUPLICATE KEY UPDATE user_id=user_id", user); e != nil {
		return
	}
	e = tx.QueryRowCtx(ctx, &a, "SELECT available_cents,held_cents,paid_cents FROM creator_cash_account WHERE user_id=? FOR UPDATE", user)
	return
}
func cashChange(ctx context.Context, tx sqlx.Session, user, key, kind, ref string, delta, held, paid int64) error {
	a, e := lockCreator(ctx, tx, user)
	if e != nil {
		return e
	}
	var count int
	e = tx.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM creator_cash_ledger WHERE event_key=?", key)
	if e != nil {
		return e
	}
	if count > 0 {
		return nil
	}
	if a.Held+held < 0 {
		return common.ErrParam
	}
	if _, e = tx.ExecCtx(ctx, "UPDATE creator_cash_account SET available_cents=available_cents+?,held_cents=held_cents+?,paid_cents=paid_cents+? WHERE user_id=?", delta, held, paid, user); e != nil {
		return e
	}
	_, e = tx.ExecCtx(ctx, "INSERT INTO creator_cash_ledger(user_id,event_key,kind,amount_cents,held_delta,reference_no,available_after) VALUES(?,?,?,?,?,?,?)", user, key, kind, delta, held, ref, a.Available+delta)
	return e
}

// Bounded worker catches missed callbacks and retries transactions safely after restart.
func StartCreatorRewards(ctx context.Context, db sqlx.SqlConn) {
	go func() {
		tick := time.NewTicker(time.Minute)
		defer tick.Stop()
		for {
			if e := ProcessRewards(ctx, db); e != nil {
				logx.Errorf("creator reward settlement: %v", e)
			}
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()
}
func ProcessRewards(ctx context.Context, db sqlx.SqlConn) error {
	var ids []int64
	if e := db.QueryRowsCtx(ctx, &ids, `SELECT r.id FROM creator_reward r JOIN diy_order o ON o.id=r.order_id WHERE r.next_attempt_at<=NOW() AND r.status IN ('pending','settled') AND (o.status='cancelled' OR o.payment_status IN ('refunded','refunding') OR EXISTS(SELECT 1 FROM askxuan_payment.refund f JOIN askxuan_payment.payment p ON p.id=f.payment_id WHERE p.order_no=o.order_no AND p.order_type='diy_order' AND f.status='success') OR (r.status='pending' AND o.payment_status='success')) ORDER BY r.next_attempt_at,r.id LIMIT 100`); e != nil {
		return e
	}
	for _, id := range ids {
		if e := processReward(ctx, db, id); e != nil {
			logx.Errorf("reward %d retry deferred: %v", id, e)
			if _, err := db.ExecCtx(ctx, "UPDATE creator_reward SET next_attempt_at=DATE_ADD(NOW(),INTERVAL 5 MINUTE),reason='结算待重试，请管理员检查奖励库存或服务状态' WHERE id=?", id); err != nil {
				return err
			}
		} else {
			if _, err := db.ExecCtx(ctx, "UPDATE creator_reward SET next_attempt_at=DATE_ADD(NOW(),INTERVAL 1 MINUTE) WHERE id=?", id); err != nil {
				return err
			}
		}
	}
	return nil
}
func processReward(ctx context.Context, db sqlx.SqlConn, id int64) error {
	return db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var r CreatorReward
		if e := tx.QueryRowCtx(ctx, &r, "SELECT "+rewardFields+" FROM creator_reward WHERE id=? FOR UPDATE", id); e != nil {
			return e
		}
		if r.Status != "pending" && r.Status != "settled" {
			return nil
		}
		var o struct {
			Status  string `db:"status"`
			Payment string `db:"payment_status"`
			Mature  bool   `db:"mature"`
			User    string `db:"user_id"`
		}
		if e := tx.QueryRowCtx(ctx, &o, "SELECT status,payment_status,user_id,update_time<=DATE_SUB(NOW(),INTERVAL ? DAY) mature FROM diy_order WHERE id=? FOR UPDATE", r.WaitDays, r.OrderID); e != nil {
			return e
		}
		var refunded int
		if e := tx.QueryRowCtx(ctx, &refunded, "SELECT COUNT(*) FROM askxuan_payment.refund f JOIN askxuan_payment.payment p ON p.id=f.payment_id WHERE p.order_no=? AND p.order_type='diy_order' AND f.status='success'", r.OrderNo); e != nil {
			return e
		}
		if refunded > 0 || o.Status == "cancelled" || o.Payment == "refunded" {
			if r.Status == "settled" {
				if e := reverseReward(ctx, tx, r); e != nil {
					return e
				}
			}
			status, reason := "cancelled", "订单取消或退款，奖励已撤销"
			if r.Kind == "coupon" && r.Status == "settled" {
				var used int
				if e := tx.QueryRowCtx(ctx, &used, "SELECT COUNT(*) FROM creator_reward_delivery d JOIN askxuan_marketing.coupon_record c ON c.id=d.coupon_record_id WHERE d.reward_id=? AND c.status='used'", id); e != nil {
					return e
				}
				if used > 0 {
					status = "review_required"
					reason = "奖励券已用于消费，退款需人工核对"
				}
			}
			_, e := tx.ExecCtx(ctx, "UPDATE creator_reward SET status=?,reason=? WHERE id=?", status, reason, id)
			return e
		}
		if r.Status != "pending" || o.Payment != "success" {
			return nil
		}
		var valid int
		if e := tx.QueryRowCtx(ctx, &valid, `SELECT COUNT(*) FROM askxuan_payment.payment WHERE order_no=? AND order_type='diy_order' AND user_id=? AND status='success' AND channel IN ('wechat','alipay','balance')`, r.OrderNo, o.User); e != nil {
			return e
		}
		if valid == 0 || r.Creator == o.User {
			_, e := tx.ExecCtx(ctx, "UPDATE creator_reward SET status='excluded',reason='模拟支付或本人购买不参与现金及权益奖励' WHERE id=?", id)
			return e
		}
		// The immutable checkout separates net material payment from platform
		// subsidy. Historical orders without a quote retain their original base.
		var materialNet int64
		netErr := tx.QueryRowCtx(ctx, &materialNet, "SELECT goods_net_cents FROM askxuan_payment.growth_quote WHERE order_type='diy_order' AND order_no=? AND status='paid' ORDER BY created_at DESC LIMIT 1", r.OrderNo)
		if netErr == nil {
			if materialNet < 0 || materialNet > r.Base {
				return common.ErrParam
			}
			if r.Kind == "cash" {
				var policy struct {
					Rate int64 `db:"rate_bps"`
					Cap  int64 `db:"cap_cents"`
				}
				if e := tx.QueryRowCtx(ctx, &policy, "SELECT rate_bps,cap_cents FROM creator_reward WHERE id=?", r.ID); e != nil {
					return e
				}
				r.Amount = RewardAmount(materialNet, policy.Rate, policy.Cap)
			}
			r.Base = materialNet
			if _, e := tx.ExecCtx(ctx, "UPDATE creator_reward SET base_cents=?,amount_cents=? WHERE id=? AND status='pending'", r.Base, r.Amount, r.ID); e != nil {
				return e
			}
		} else if !errors.Is(netErr, sqlx.ErrNotFound) {
			return netErr
		}
		if o.Status != "completed" || !o.Mature {
			return nil
		}
		var pendingRefund int
		if e := tx.QueryRowCtx(ctx, &pendingRefund, "SELECT COUNT(*) FROM askxuan_payment.refund f JOIN askxuan_payment.payment p ON p.id=f.payment_id WHERE p.order_no=? AND p.order_type='diy_order' AND f.status IN ('pending','processing')", r.OrderNo); e != nil {
			return e
		}
		if pendingRefund > 0 {
			return nil
		}
		if e := grantReward(ctx, tx, r); e != nil {
			return e
		}
		_, e := tx.ExecCtx(ctx, "UPDATE creator_reward SET status='settled',reason='',settled_at=NOW() WHERE id=?", id)
		return e
	})
}
func grantReward(ctx context.Context, tx sqlx.Session, r CreatorReward) error {
	if r.Kind == "cash" {
		return cashChange(ctx, tx, r.Creator, fmt.Sprintf("reward:%d", r.ID), "reward", r.OrderNo, r.Amount, 0, 0)
	}
	return grantNonCashReward(ctx, tx, r, false)
}
func reverseReward(ctx context.Context, tx sqlx.Session, r CreatorReward) error {
	if r.Kind == "cash" {
		return cashChange(ctx, tx, r.Creator, fmt.Sprintf("reverse:%d", r.ID), "refund_reversal", r.OrderNo, -r.Amount, 0, 0)
	}
	return grantNonCashReward(ctx, tx, r, true)
}

type CreatorWithdrawal struct {
	ID        int64  `db:"id" json:"id"`
	User      string `db:"user_id" json:"userId"`
	Key       string `db:"request_key" json:"requestKey"`
	Amount    int64  `db:"amount_cents" json:"amountCents"`
	Holder    string `db:"holder_name" json:"holderName"`
	Bank      string `db:"bank_name" json:"bankName"`
	Account   string `db:"account_number" json:"accountNumber"`
	Status    string `db:"status" json:"status"`
	Reference string `db:"transfer_reference" json:"transferReference"`
	Note      string `db:"note" json:"note"`
	Created   string `db:"created_at" json:"createdAt"`
}

const withdrawalFields = "id,user_id,request_key,amount_cents,holder_name,bank_name,account_number,status,COALESCE(transfer_reference,'') transfer_reference,note,DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s') created_at"

func ApplyCreatorWithdrawal(ctx context.Context, db sqlx.SqlConn, user string, w CreatorWithdrawal, secret string) (out int64, err error) {
	if w.Amount <= 0 || w.Amount > 1000000 || len(w.Key) < 16 || len(w.Key) > 64 || len(strings.TrimSpace(w.Holder)) < 2 || len(w.Holder) > 100 || len(strings.TrimSpace(w.Bank)) < 2 || len(w.Bank) > 150 || len(w.Account) < 6 || len(w.Account) > 100 {
		return 0, common.ErrParam
	}
	err = db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		a, e := lockCreator(ctx, tx, user)
		if e != nil {
			return e
		}
		var old CreatorWithdrawal
		e = tx.QueryRowCtx(ctx, &old, "SELECT "+withdrawalFields+" FROM creator_withdrawal WHERE user_id=? AND request_key=?", user, w.Key)
		if e == nil {
			decoded, err := OpenPayout(secret, user, old.Account)
			if err != nil {
				return err
			}
			if old.Amount != w.Amount || decoded != w.Account || old.Holder != w.Holder || old.Bank != w.Bank {
				return common.ErrParam
			}
			out = old.ID
			return nil
		}
		if !errors.Is(e, sqlx.ErrNotFound) {
			return e
		}
		p, e := ReadRewardPolicy(ctx, tx)
		if e != nil {
			return e
		}
		if w.Amount < p.MinWithdraw || a.Available < w.Amount {
			return common.NewBizError(40901, "可提现金额不足或未达到最低提现金额")
		}
		encrypted, e := SealPayout(secret, user, w.Account)
		if e != nil {
			return e
		}
		result, e := tx.ExecCtx(ctx, "INSERT INTO creator_withdrawal(user_id,request_key,amount_cents,holder_name,bank_name,account_number) VALUES(?,?,?,?,?,?)", user, w.Key, w.Amount, strings.TrimSpace(w.Holder), strings.TrimSpace(w.Bank), encrypted)
		if e != nil {
			return e
		}
		out, e = result.LastInsertId()
		if e != nil {
			return e
		}
		return cashChange(ctx, tx, user, fmt.Sprintf("withdraw:%d", out), "withdraw_hold", fmt.Sprint(out), -w.Amount, w.Amount, 0)
	})
	return
}
func ReviewCreatorWithdrawal(ctx context.Context, db sqlx.SqlConn, id int64, action, reference, note, operator string) error {
	if len(note) > 500 || len(reference) > 128 || id < 1 {
		return common.ErrParam
	}
	return db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var user string
		if e := tx.QueryRowCtx(ctx, &user, "SELECT user_id FROM creator_withdrawal WHERE id=?", id); e != nil {
			return e
		}
		a, e := lockCreator(ctx, tx, user)
		if e != nil {
			return e
		}
		var w CreatorWithdrawal
		if e = tx.QueryRowCtx(ctx, &w, "SELECT "+withdrawalFields+" FROM creator_withdrawal WHERE id=? FOR UPDATE", id); e != nil {
			return e
		}
		next := ""
		delta, held, paid := int64(0), int64(0), int64(0)
		switch action {
		case "approve":
			if w.Status == "pending" && a.Available >= 0 {
				next = "approved"
			}
		case "reject":
			if w.Status == "pending" || w.Status == "approved" {
				if strings.TrimSpace(note) == "" {
					return common.ErrParam
				}
				next = "rejected"
				delta = w.Amount
				held = -w.Amount
			}
		case "processing":
			if w.Status == "approved" && a.Available >= 0 {
				next = "processing"
			}
		case "paid":
			if w.Status == "processing" && strings.TrimSpace(reference) != "" && strings.TrimSpace(note) != "" {
				next = "paid"
				held = -w.Amount
				paid = w.Amount
			}
		case "failed":
			if w.Status == "processing" && strings.TrimSpace(note) != "" {
				next = "failed"
				delta = w.Amount
				held = -w.Amount
			}
		}
		if next == "" {
			return common.NewBizError(40901, "提现状态已变化，或存在退款待抵扣；请刷新核对")
		}
		if e = cashChange(ctx, tx, user, fmt.Sprintf("withdraw:%d:%s", id, next), "withdraw_"+next, fmt.Sprint(id), delta, held, paid); e != nil {
			return e
		}
		var ref interface{}
		if next == "paid" {
			ref = strings.TrimSpace(reference)
		}
		if _, e = tx.ExecCtx(ctx, "UPDATE creator_withdrawal SET status=?,transfer_reference=?,note=?,operator_id=? WHERE id=?", next, ref, note, operator, id); e != nil {
			return e
		}
		return rewardAudit(ctx, tx, operator, "withdraw_"+next, fmt.Sprint(id), note)
	})
}
