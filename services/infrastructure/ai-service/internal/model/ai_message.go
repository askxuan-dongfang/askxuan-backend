package model

import (
	"sync"
)

// 消息角色
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
)

// AIMessage 对话消息
type AIMessage struct {
	Id        int64  `json:"id"`
	SessionId int64  `json:"sessionId"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Tokens    int    `json:"tokens"`
	CreatedAt string `json:"createdAt"`
}

// ---- 内存存储 ----

type messageStore struct {
	mu   sync.RWMutex
	list []AIMessage
	seq  int64
}

var globalMessageStore = &messageStore{
	list: []AIMessage{
		{Id: 1, SessionId: 1, Role: RoleUser, Content: "请帮我看看今年的事业运势", Tokens: 0, CreatedAt: "2026-06-30 09:01:00"},
		{Id: 2, SessionId: 1, Role: RoleAssistant, Content: "好的，请问您的出生年月日时是？", Tokens: 0, CreatedAt: "2026-06-30 09:01:05"},
	},
	seq: 2,
}

// ===== Message =====

// ListMessagesBySession 查询会话消息列表（按时间正序）
func ListMessagesBySession(sessionId int64) []AIMessage {
	globalMessageStore.mu.RLock()
	defer globalMessageStore.mu.RUnlock()
	out := make([]AIMessage, 0)
	for _, m := range globalMessageStore.list {
		if m.SessionId == sessionId {
			out = append(out, m)
		}
	}
	return out
}

// InsertMessage 新增消息
func InsertMessage(m AIMessage) AIMessage {
	globalMessageStore.mu.Lock()
	defer globalMessageStore.mu.Unlock()
	globalMessageStore.seq++
	m.Id = globalMessageStore.seq
	m.CreatedAt = nowStr()
	globalMessageStore.list = append(globalMessageStore.list, m)
	return m
}
