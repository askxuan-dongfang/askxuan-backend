package model

import (
	"sync"
)

// 推荐位类型
const (
	RecommendTypeTemple  = "temple"
	RecommendTypeMaster  = "master"
	RecommendTypeProduct = "product"
)

// Recommend 推荐位
type Recommend struct {
	Id        int64  `json:"id"`
	Type      string `json:"type"`
	TargetId  string `json:"targetId"`
	Sort      int    `json:"sort"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
}

// ---- 内存存储（MVP-4 阶段不连 DB）----

type recommendStore struct {
	mu   sync.RWMutex
	list []Recommend
	seq  int64
}

var globalRecommendStore = &recommendStore{
	list: []Recommend{
		{Id: 1, Type: RecommendTypeTemple, TargetId: "T001", Sort: 1, Status: StatusEnabled, CreatedAt: "2026-06-28 10:00:00"},
		{Id: 2, Type: RecommendTypeTemple, TargetId: "T003", Sort: 2, Status: StatusEnabled, CreatedAt: "2026-06-28 10:00:00"},
		{Id: 3, Type: RecommendTypeMaster, TargetId: "M001", Sort: 1, Status: StatusEnabled, CreatedAt: "2026-06-28 10:00:00"},
	},
	seq: 3,
}

// ===== Recommend CRUD =====

// ListRecommends 推荐位列表
func ListRecommends(rType, status string, page, size int) ([]Recommend, int64) {
	globalRecommendStore.mu.RLock()
	defer globalRecommendStore.mu.RUnlock()
	filtered := make([]Recommend, 0, len(globalRecommendStore.list))
	for _, r := range globalRecommendStore.list {
		if rType != "" && r.Type != rType {
			continue
		}
		if status != "" && r.Status != status {
			continue
		}
		filtered = append(filtered, r)
	}
	return paginateRecommend(filtered, page, size)
}

// UpdateRecommend 更新推荐位
func UpdateRecommend(id int64, r Recommend) (Recommend, bool) {
	globalRecommendStore.mu.Lock()
	defer globalRecommendStore.mu.Unlock()
	for i := range globalRecommendStore.list {
		if globalRecommendStore.list[i].Id == id {
			if r.Type != "" {
				globalRecommendStore.list[i].Type = r.Type
			}
			if r.TargetId != "" {
				globalRecommendStore.list[i].TargetId = r.TargetId
			}
			if r.Status != "" {
				globalRecommendStore.list[i].Status = r.Status
			}
			globalRecommendStore.list[i].Sort = r.Sort
			return globalRecommendStore.list[i], true
		}
	}
	return Recommend{}, false
}

func paginateRecommend(in []Recommend, page, size int) ([]Recommend, int64) {
	total := int64(len(in))
	start, end := pageRange(page, size, len(in))
	return in[start:end], total
}
