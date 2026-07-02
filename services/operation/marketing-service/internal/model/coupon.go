package model

import (
	"fmt"
	"sync"
	"time"
)

// 优惠券类型
const (
	CouponTypeFullReduce = "full_reduce" // 满减
	CouponTypeDiscount   = "discount"    // 折扣
	CouponTypeNewUser    = "new_user"    // 新人专享
	CouponTypeCategory   = "category"    // 品类券
)

// Coupon 优惠券
type Coupon struct {
	Id            int64   `json:"id"`
	CouponNo      string  `json:"couponNo"`
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	Value         float64 `json:"value"`
	MinAmount     float64 `json:"minAmount"`
	CategoryId    string  `json:"categoryId"`
	StartTime     string  `json:"startTime"`
	EndTime       string  `json:"endTime"`
	TotalCount    int     `json:"totalCount"`
	ReceivedCount int     `json:"receivedCount"`
	Status        string  `json:"status"`
	CreatedAt     string  `json:"createdAt"`
}

// ---- 内存存储（MVP-4 阶段不连 DB）----

type couponStore struct {
	mu   sync.RWMutex
	list []Coupon
	seq  int64
}

var globalCouponStore = &couponStore{
	list: []Coupon{
		{Id: 1, CouponNo: "C20260700001", Name: "新人满100减20", Type: CouponTypeNewUser, Value: 20, MinAmount: 100, StartTime: "2026-07-01 00:00:00", EndTime: "2026-12-31 23:59:59", TotalCount: 1000, ReceivedCount: 12, Status: StatusEnabled, CreatedAt: "2026-06-28 10:00:00"},
		{Id: 2, CouponNo: "C20260700002", Name: "法事8折券", Type: CouponTypeDiscount, Value: 0.8, MinAmount: 0, StartTime: "2026-07-01 00:00:00", EndTime: "2026-08-31 23:59:59", TotalCount: 500, ReceivedCount: 3, Status: StatusEnabled, CreatedAt: "2026-06-28 10:00:00"},
	},
	seq: 2,
}

// ===== Coupon CRUD =====

// FindCouponByID 按 ID 查询优惠券
func FindCouponByID(id int64) (Coupon, bool) {
	globalCouponStore.mu.RLock()
	defer globalCouponStore.mu.RUnlock()
	for _, c := range globalCouponStore.list {
		if c.Id == id {
			return c, true
		}
	}
	return Coupon{}, false
}

// ListCoupons 优惠券列表
func ListCoupons(status, cType string, page, size int) ([]Coupon, int64) {
	globalCouponStore.mu.RLock()
	defer globalCouponStore.mu.RUnlock()
	filtered := make([]Coupon, 0, len(globalCouponStore.list))
	for _, c := range globalCouponStore.list {
		if status != "" && c.Status != status {
			continue
		}
		if cType != "" && c.Type != cType {
			continue
		}
		filtered = append(filtered, c)
	}
	return paginateCoupon(filtered, page, size)
}

// InsertCoupon 新建优惠券
func InsertCoupon(c Coupon) Coupon {
	globalCouponStore.mu.Lock()
	defer globalCouponStore.mu.Unlock()
	globalCouponStore.seq++
	c.Id = globalCouponStore.seq
	c.CouponNo = fmt.Sprintf("C%s%05d", time.Now().Format("200601"), globalCouponStore.seq)
	if c.Status == "" {
		c.Status = StatusEnabled
	}
	c.CreatedAt = nowStr()
	globalCouponStore.list = append(globalCouponStore.list, c)
	return c
}

// UpdateCoupon 更新优惠券
func UpdateCoupon(id int64, c Coupon) (Coupon, bool) {
	globalCouponStore.mu.Lock()
	defer globalCouponStore.mu.Unlock()
	for i := range globalCouponStore.list {
		if globalCouponStore.list[i].Id == id {
			if c.Name != "" {
				globalCouponStore.list[i].Name = c.Name
			}
			if c.Type != "" {
				globalCouponStore.list[i].Type = c.Type
			}
			if c.StartTime != "" {
				globalCouponStore.list[i].StartTime = c.StartTime
			}
			if c.EndTime != "" {
				globalCouponStore.list[i].EndTime = c.EndTime
			}
			if c.Status != "" {
				globalCouponStore.list[i].Status = c.Status
			}
			globalCouponStore.list[i].Value = c.Value
			globalCouponStore.list[i].MinAmount = c.MinAmount
			globalCouponStore.list[i].TotalCount = c.TotalCount
			return globalCouponStore.list[i], true
		}
	}
	return Coupon{}, false
}

func paginateCoupon(in []Coupon, page, size int) ([]Coupon, int64) {
	total := int64(len(in))
	start, end := pageRange(page, size, len(in))
	return in[start:end], total
}
