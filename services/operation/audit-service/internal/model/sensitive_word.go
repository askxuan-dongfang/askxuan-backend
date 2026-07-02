package model

import (
	"sync"
)

// 敏感词状态
const (
	SensitiveWordEnabled  = "enabled"
	SensitiveWordDisabled = "disabled"
)

// SensitiveWord 敏感词结构体
type SensitiveWord struct {
	Id         int64  `json:"id"`
	Word       string `json:"word"`
	Category   string `json:"category"`
	Status     string `json:"status"`
	CreateTime string `json:"createTime"`
}

// ---- 内存存储（MVP 阶段不连 DB）----

type sensitiveWordStore struct {
	mu   sync.RWMutex
	list []SensitiveWord
	seq  int64
}

var globalSensitiveWordStore = &sensitiveWordStore{
	list: []SensitiveWord{
		{Id: 1, Word: "邪教", Category: "religious", Status: SensitiveWordEnabled, CreateTime: "2026-07-01 00:00:00"},
		{Id: 2, Word: "反动", Category: "political", Status: SensitiveWordEnabled, CreateTime: "2026-07-01 00:00:00"},
		{Id: 3, Word: "色情", Category: "vulgar", Status: SensitiveWordEnabled, CreateTime: "2026-07-01 00:00:00"},
		{Id: 4, Word: "加微信", Category: "advertising", Status: SensitiveWordEnabled, CreateTime: "2026-07-01 00:00:00"},
		{Id: 5, Word: "代购", Category: "advertising", Status: SensitiveWordDisabled, CreateTime: "2026-07-01 00:00:00"},
	},
	seq: 5,
}

// ListSensitiveWords 查询敏感词列表
func ListSensitiveWords(category, status, keyword string, page, size int) ([]SensitiveWord, int64) {
	globalSensitiveWordStore.mu.RLock()
	defer globalSensitiveWordStore.mu.RUnlock()

	filtered := make([]SensitiveWord, 0, len(globalSensitiveWordStore.list))
	for _, sw := range globalSensitiveWordStore.list {
		if category != "" && sw.Category != category {
			continue
		}
		if status != "" && sw.Status != status {
			continue
		}
		if keyword != "" && !contains(sw.Word, keyword) {
			continue
		}
		filtered = append(filtered, sw)
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

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
