package model

import (
	"fmt"
	"sync"
	"time"
)

// 评价状态常量
const (
	ReviewStatusNormal = "normal" // 正常显示
	ReviewStatusHidden = "hidden" // 已隐藏
)

// 评价目标类型
const (
	TargetTypeBooking   = "booking"
	TargetTypeDiyOrder  = "diy_order"
	TargetTypeShopOrder = "shop_order"
)

// Review 评价结构体
type Review struct {
	Id         int64  `json:"id"`
	ReviewNo   string `json:"reviewNo"`
	UserId     string `json:"userId"`
	TargetType string `json:"targetType"`
	TargetId   string `json:"targetId"`
	Rating     int    `json:"rating"`
	Content    string `json:"content"`
	Images     string `json:"images"`
	Status     string `json:"status"`
	CreateTime string `json:"createTime"`
}

// ---- 内存存储（MVP 阶段不连 DB）----

type reviewStore struct {
	mu   sync.RWMutex
	list []Review
	seq  int64
}

var globalReviewStore = &reviewStore{
	list: []Review{
		{
			Id:         1,
			ReviewNo:   "RV20260620001",
			UserId:     "U001",
			TargetType: TargetTypeBooking,
			TargetId:   "B20260615003",
			Rating:     5,
			Content:    "清风道长非常专业，化太岁仪式庄重，感觉很安心。",
			Images:     `["https://oss.askxuan.com/rv/1.jpg"]`,
			Status:     ReviewStatusNormal,
			CreateTime: "2026-06-20 18:00:00",
		},
		{
			Id:         2,
			ReviewNo:   "RV20260625002",
			UserId:     "U002",
			TargetType: TargetTypeBooking,
			TargetId:   "B20260628002",
			Rating:     4,
			Content:    "释延心法师超度法事很用心，整体体验不错。",
			Images:     `[]`,
			Status:     ReviewStatusNormal,
			CreateTime: "2026-06-25 20:30:00",
		},
		{
			Id:         3,
			ReviewNo:   "RV20260628003",
			UserId:     "U001",
			TargetType: TargetTypeShopOrder,
			TargetId:   "SO20260620001",
			Rating:     5,
			Content:    "小叶紫檀手串品质很好，包装精美，非常满意！",
			Images:     `["https://oss.askxuan.com/rv/2.jpg","https://oss.askxuan.com/rv/3.jpg"]`,
			Status:     ReviewStatusNormal,
			CreateTime: "2026-06-28 10:00:00",
		},
	},
	seq: 3,
}

// ListReviews 查询评价列表，支持按 targetType/targetId/userId/rating 筛选 + 分页
func ListReviews(targetType, targetId, userId string, rating int, status string, page, size int) ([]Review, int64) {
	globalReviewStore.mu.RLock()
	defer globalReviewStore.mu.RUnlock()

	filtered := make([]Review, 0, len(globalReviewStore.list))
	for _, r := range globalReviewStore.list {
		if targetType != "" && r.TargetType != targetType {
			continue
		}
		if targetId != "" && r.TargetId != targetId {
			continue
		}
		if userId != "" && r.UserId != userId {
			continue
		}
		if rating > 0 && r.Rating != rating {
			continue
		}
		if status != "" && r.Status != status {
			continue
		}
		filtered = append(filtered, r)
	}

	total := int64(len(filtered))
	start := (page - 1) * size
	if start < 0 {
		start = 0
	}
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + size
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[start:end], total
}

// FindReviewByID 按ID查询评价
func FindReviewByID(id int64) (Review, bool) {
	globalReviewStore.mu.RLock()
	defer globalReviewStore.mu.RUnlock()
	for _, r := range globalReviewStore.list {
		if r.Id == id {
			return r, true
		}
	}
	return Review{}, false
}

// CreateReview 新建评价，seq 自增，生成 reviewNo（格式 R{YYYYMMDD}{seq:06d}），默认 status=normal，设置 createTime
func CreateReview(r Review) Review {
	globalReviewStore.mu.Lock()
	defer globalReviewStore.mu.Unlock()

	globalReviewStore.seq++
	r.Id = globalReviewStore.seq
	if r.Status == "" {
		r.Status = ReviewStatusNormal
	}
	r.ReviewNo = fmt.Sprintf("R%s%06d", time.Now().Format("20060102"), r.Id)
	r.CreateTime = time.Now().Format("2006-01-02 15:04:05")
	globalReviewStore.list = append(globalReviewStore.list, r)
	return r
}

// UpdateReviewStatus 更新评价状态，找到并更新返回 true，未找到返回 false
func UpdateReviewStatus(id int64, status string) bool {
	globalReviewStore.mu.Lock()
	defer globalReviewStore.mu.Unlock()

	for i := range globalReviewStore.list {
		if globalReviewStore.list[i].Id == id {
			globalReviewStore.list[i].Status = status
			return true
		}
	}
	return false
}
