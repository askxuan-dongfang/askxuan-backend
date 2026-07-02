package model

import (
	"sync"
	"time"
)

// AuditLog 审核日志结构体
type AuditLog struct {
	Id         int64  `json:"id"`
	AuditId    int64  `json:"auditId"`
	Action     string `json:"action"`
	OperatorId string `json:"operatorId"`
	Remark     string `json:"remark"`
	CreateTime string `json:"createTime"`
}

// ---- 内存存储（MVP 阶段不连 DB）----

type auditLogStore struct {
	mu   sync.RWMutex
	list []AuditLog
	seq  int64
}

var globalAuditLogStore = &auditLogStore{}

// InsertAuditLog 新增审核日志，seq 自增，设置 createTime 后追加到 store
func InsertAuditLog(log AuditLog) AuditLog {
	globalAuditLogStore.mu.Lock()
	defer globalAuditLogStore.mu.Unlock()
	globalAuditLogStore.seq++
	log.Id = globalAuditLogStore.seq
	log.CreateTime = time.Now().Format("2006-01-02 15:04:05")
	globalAuditLogStore.list = append(globalAuditLogStore.list, log)
	return log
}

// ListAuditLogByAuditID 按 auditId 查询审核日志列表
func ListAuditLogByAuditID(auditId int64) []AuditLog {
	globalAuditLogStore.mu.RLock()
	defer globalAuditLogStore.mu.RUnlock()
	result := make([]AuditLog, 0)
	for _, l := range globalAuditLogStore.list {
		if l.AuditId == auditId {
			result = append(result, l)
		}
	}
	return result
}
