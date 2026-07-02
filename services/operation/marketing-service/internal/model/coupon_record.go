package model

import (
	"fmt"
	"sync"
)

// 优惠券领取记录状态
const (
	CouponRecordUnused  = "unused"
	CouponRecordUsed    = "used"
	CouponRecordExpired = "expired"
)

// CouponRecord 优惠券领取记录
type CouponRecord struct {
	Id        int64  `json:"id"`
	CouponId  int64  `json:"couponId"`
	CouponNo  string `json:"couponNo"`
	UserId    string `json:"userId"`
	Status    string `json:"status"`
	OrderNo   string `json:"orderNo"`
	UseTime   string `json:"useTime"`
	CreatedAt string `json:"createdAt"`
}

// ---- 内存存储（MVP-4 阶段不连 DB）----

type couponRecordStore struct {
	mu   sync.RWMutex
	list []CouponRecord
	seq  int64
}

var globalCouponRecordStore = &couponRecordStore{
	list: []CouponRecord{
		{Id: 1, CouponId: 1, CouponNo: "C20260700001", UserId: "U001", Status: CouponRecordUnused, CreatedAt: "2026-06-29 09:00:00"},
	},
	seq: 1,
}

// ReceiveCoupon 用户领取优惠券（库存 +1，写领取记录）
// 注意：跨 couponStore 与 couponRecordStore 操作，按 coupon → record 顺序加锁避免死锁
func ReceiveCoupon(couponId int64, userId string) (CouponRecord, error) {
	globalCouponStore.mu.Lock()
	defer globalCouponStore.mu.Unlock()
	for i := range globalCouponStore.list {
		if globalCouponStore.list[i].Id != couponId {
			continue
		}
		if globalCouponStore.list[i].Status != StatusEnabled {
			return CouponRecord{}, fmt.Errorf("优惠券已下架")
		}
		if globalCouponStore.list[i].ReceivedCount >= globalCouponStore.list[i].TotalCount {
			return CouponRecord{}, fmt.Errorf("优惠券已领完")
		}
		globalCouponStore.list[i].ReceivedCount++
		globalCouponRecordStore.mu.Lock()
		defer globalCouponRecordStore.mu.Unlock()
		globalCouponRecordStore.seq++
		rec := CouponRecord{
			Id:        globalCouponRecordStore.seq,
			CouponId:  globalCouponStore.list[i].Id,
			CouponNo:  globalCouponStore.list[i].CouponNo,
			UserId:    userId,
			Status:    CouponRecordUnused,
			CreatedAt: nowStr(),
		}
		globalCouponRecordStore.list = append(globalCouponRecordStore.list, rec)
		return rec, nil
	}
	return CouponRecord{}, fmt.Errorf("优惠券不存在")
}

// ListMyCoupons 我的优惠券列表
func ListMyCoupons(userId, status string, page, size int) ([]CouponRecord, int64) {
	globalCouponRecordStore.mu.RLock()
	defer globalCouponRecordStore.mu.RUnlock()
	filtered := make([]CouponRecord, 0, len(globalCouponRecordStore.list))
	for _, r := range globalCouponRecordStore.list {
		if r.UserId != userId {
			continue
		}
		if status != "" && r.Status != status {
			continue
		}
		filtered = append(filtered, r)
	}
	return paginateRecord(filtered, page, size)
}

func paginateRecord(in []CouponRecord, page, size int) ([]CouponRecord, int64) {
	total := int64(len(in))
	start, end := pageRange(page, size, len(in))
	return in[start:end], total
}
