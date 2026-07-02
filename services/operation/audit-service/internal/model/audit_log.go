package model

import (
	"sync"
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
}

var globalAuditLogStore = &auditLogStore{}
