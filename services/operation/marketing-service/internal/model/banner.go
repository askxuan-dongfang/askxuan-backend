package model

import (
	"sync"
	"time"
)

// 通用启停状态
const (
	StatusEnabled  = "enabled"
	StatusDisabled = "disabled"
)

// Banner 跳转类型
const (
	LinkTypeTemple    = "temple"
	LinkTypeMaster    = "master"
	LinkTypeProduct   = "product"
	LinkTypeDiy       = "diy"
	LinkTypeAdLanding = "ad_landing"
)

// Banner 首页轮播
type Banner struct {
	Id        int64  `json:"id"`
	Title     string `json:"title"`
	ImageUrl  string `json:"imageUrl"`
	LinkType  string `json:"linkType"`
	LinkValue string `json:"linkValue"`
	Sort      int    `json:"sort"`
	Status    string `json:"status"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	CreatedAt string `json:"createdAt"`
}

// ---- 内存存储（MVP-4 阶段不连 DB）----

type bannerStore struct {
	mu   sync.RWMutex
	list []Banner
	seq  int64
}

var globalBannerStore = &bannerStore{
	list: []Banner{
		{Id: 1, Title: "灵隐寺祈福法会", ImageUrl: "/banners/lingyin.jpg", LinkType: LinkTypeTemple, LinkValue: "T001", Sort: 1, Status: StatusEnabled, StartTime: "2026-07-01 00:00:00", EndTime: "2026-07-31 23:59:59", CreatedAt: "2026-06-28 10:00:00"},
		{Id: 2, Title: "新人首单立减", ImageUrl: "/banners/newuser.jpg", LinkType: LinkTypeAdLanding, LinkValue: "/promo/newuser", Sort: 2, Status: StatusEnabled, StartTime: "2026-07-01 00:00:00", EndTime: "2026-12-31 23:59:59", CreatedAt: "2026-06-28 10:05:00"},
	},
	seq: 2,
}

// nowStr 当前时间字符串
func nowStr() string { return time.Now().Format("2006-01-02 15:04:05") }

// ===== Banner CRUD =====

// ListBanners Banner 列表，支持 status 筛选 + 分页
func ListBanners(status string, page, size int) ([]Banner, int64) {
	globalBannerStore.mu.RLock()
	defer globalBannerStore.mu.RUnlock()
	filtered := make([]Banner, 0, len(globalBannerStore.list))
	for _, b := range globalBannerStore.list {
		if status != "" && b.Status != status {
			continue
		}
		filtered = append(filtered, b)
	}
	return paginateBanner(filtered, page, size)
}

// InsertBanner 新建 Banner
func InsertBanner(b Banner) Banner {
	globalBannerStore.mu.Lock()
	defer globalBannerStore.mu.Unlock()
	globalBannerStore.seq++
	b.Id = globalBannerStore.seq
	if b.Status == "" {
		b.Status = StatusEnabled
	}
	b.CreatedAt = nowStr()
	globalBannerStore.list = append(globalBannerStore.list, b)
	return b
}

// UpdateBanner 更新 Banner
func UpdateBanner(id int64, b Banner) (Banner, bool) {
	globalBannerStore.mu.Lock()
	defer globalBannerStore.mu.Unlock()
	for i := range globalBannerStore.list {
		if globalBannerStore.list[i].Id == id {
			if b.Title != "" {
				globalBannerStore.list[i].Title = b.Title
			}
			if b.ImageUrl != "" {
				globalBannerStore.list[i].ImageUrl = b.ImageUrl
			}
			if b.LinkType != "" {
				globalBannerStore.list[i].LinkType = b.LinkType
			}
			if b.LinkValue != "" {
				globalBannerStore.list[i].LinkValue = b.LinkValue
			}
			if b.Status != "" {
				globalBannerStore.list[i].Status = b.Status
			}
			if b.StartTime != "" {
				globalBannerStore.list[i].StartTime = b.StartTime
			}
			if b.EndTime != "" {
				globalBannerStore.list[i].EndTime = b.EndTime
			}
			globalBannerStore.list[i].Sort = b.Sort
			return globalBannerStore.list[i], true
		}
	}
	return Banner{}, false
}

func paginateBanner(in []Banner, page, size int) ([]Banner, int64) {
	total := int64(len(in))
	start, end := pageRange(page, size, len(in))
	return in[start:end], total
}

// pageRange 分页区间计算（包内共享）
func pageRange(page, size, n int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	start := (page - 1) * size
	if start < 0 {
		start = 0
	}
	if start > n {
		start = n
	}
	end := start + size
	if end > n {
		end = n
	}
	return start, end
}
