package model

import (
	"sync"
)

// FinanceLog 财务流水结构体
type FinanceLog struct {
	Id           int64   `json:"id"`
	SettlementId int64   `json:"settlementId"`
	Amount       float64 `json:"amount"`
	Type         string  `json:"type"`
	Description  string  `json:"description"`
	CreateTime   string  `json:"createTime"`
}

// ---- 内存存储（MVP 阶段不连 DB）----

type financeLogStore struct {
	mu   sync.RWMutex
	list []FinanceLog
}

var globalFinanceLogStore = &financeLogStore{}
