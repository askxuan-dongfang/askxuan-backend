package model

import (
	"sync"
	"time"
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
	seq  int64
}

var globalFinanceLogStore = &financeLogStore{}

// InsertFinanceLog 插入财务流水
func InsertFinanceLog(log FinanceLog) FinanceLog {
	globalFinanceLogStore.mu.Lock()
	defer globalFinanceLogStore.mu.Unlock()
	globalFinanceLogStore.seq++
	log.Id = globalFinanceLogStore.seq
	log.CreateTime = time.Now().Format("2006-01-02 15:04:05")
	globalFinanceLogStore.list = append(globalFinanceLogStore.list, log)
	return log
}

// SumByType 按类型聚合金额
func SumByType(start, end string) map[string]float64 {
	globalFinanceLogStore.mu.RLock()
	defer globalFinanceLogStore.mu.RUnlock()
	result := map[string]float64{}
	for _, l := range globalFinanceLogStore.list {
		result[l.Type] += l.Amount
	}
	return result
}
