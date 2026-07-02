package model

import (
	"fmt"
	"sync"
	"time"
)

// 会话状态
const (
	SessionStatusActive = "active"
	SessionStatusClosed = "closed"
)

// AISession 对话会话
type AISession struct {
	Id        int64  `json:"id"`
	SessionNo string `json:"sessionNo"`
	UserId    string `json:"userId"`
	SkillCode string `json:"skillCode"`
	Status    string `json:"status"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// ---- 内存存储 ----

type sessionStore struct {
	mu   sync.RWMutex
	list []AISession
	seq  int64
}

var globalSessionStore = &sessionStore{
	list: []AISession{
		{Id: 1, SessionNo: "AI20260630001", UserId: "U001", SkillCode: SkillCodeBazi, Status: SessionStatusActive, CreatedAt: "2026-06-30 09:00:00", UpdatedAt: "2026-06-30 09:00:00"},
	},
	seq: 1,
}

func nowStr() string { return time.Now().Format("2006-01-02 15:04:05") }

// ===== Session =====

// InsertSession 创建会话
func InsertSession(s AISession) AISession {
	globalSessionStore.mu.Lock()
	defer globalSessionStore.mu.Unlock()
	globalSessionStore.seq++
	s.Id = globalSessionStore.seq
	s.SessionNo = fmt.Sprintf("AI%s%03d", time.Now().Format("20060102"), globalSessionStore.seq)
	s.Status = SessionStatusActive
	ts := nowStr()
	s.CreatedAt = ts
	s.UpdatedAt = ts
	globalSessionStore.list = append(globalSessionStore.list, s)
	return s
}

// ListSessions 用户会话列表
func ListSessions(userId, status string, page, size int) ([]AISession, int64) {
	globalSessionStore.mu.RLock()
	defer globalSessionStore.mu.RUnlock()
	filtered := make([]AISession, 0, len(globalSessionStore.list))
	for _, s := range globalSessionStore.list {
		if s.UserId != userId {
			continue
		}
		if status != "" && s.Status != status {
			continue
		}
		filtered = append(filtered, s)
	}
	total := int64(len(filtered))
	start, end := pageRange(page, size, len(filtered))
	return filtered[start:end], total
}

// FindSessionByID 按ID查询会话
func FindSessionByID(id int64) (AISession, bool) {
	globalSessionStore.mu.RLock()
	defer globalSessionStore.mu.RUnlock()
	for _, s := range globalSessionStore.list {
		if s.Id == id {
			return s, true
		}
	}
	return AISession{}, false
}

// CloseSession 关闭会话
func CloseSession(id int64) (AISession, bool) {
	globalSessionStore.mu.Lock()
	defer globalSessionStore.mu.Unlock()
	for i := range globalSessionStore.list {
		if globalSessionStore.list[i].Id == id {
			globalSessionStore.list[i].Status = SessionStatusClosed
			globalSessionStore.list[i].UpdatedAt = nowStr()
			return globalSessionStore.list[i], true
		}
	}
	return AISession{}, false
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
