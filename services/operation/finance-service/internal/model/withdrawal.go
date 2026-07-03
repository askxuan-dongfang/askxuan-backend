package model

import (
	"fmt"
	"sync"
	"time"
)

// 提现状态常量（参照 state-machines.md 第6节）
const (
	WithdrawalPending    = "pending"    // 待审核
	WithdrawalApproved   = "approved"   // 已审核
	WithdrawalProcessing = "processing" // 打款中
	WithdrawalSuccess    = "success"    // 打款成功
	WithdrawalFailed     = "failed"     // 打款失败
	WithdrawalRejected   = "rejected"   // 已拒绝
)

// withdrawalTransitions 提现状态机合法流转（参照 state-machines.md 6.2）
var withdrawalTransitions = map[string]map[string]bool{
	WithdrawalPending: {
		WithdrawalApproved: true,
		WithdrawalRejected: true,
	},
	WithdrawalApproved: {
		WithdrawalProcessing: true,
	},
	WithdrawalProcessing: {
		WithdrawalSuccess: true,
		WithdrawalFailed:  true,
	},
	WithdrawalFailed: {
		WithdrawalProcessing: true,
	},
}

// CanTransitWithdrawal 校验提现状态流转是否合法
func CanTransitWithdrawal(from, to string) bool {
	if from == to {
		return false
	}
	allowed, ok := withdrawalTransitions[from]
	if !ok {
		return false
	}
	return allowed[to]
}

// Withdrawal 提现申请结构体
type Withdrawal struct {
	Id            int64   `json:"id"`
	WithdrawalNo  string  `json:"withdrawalNo"`
	ApplicantType string  `json:"applicantType"`
	ApplicantId   string  `json:"applicantId"`
	Amount        float64 `json:"amount"`
	BankCard      string  `json:"bankCard"`
	Status        string  `json:"status"`
	AuditTime     string  `json:"auditTime"`
	ProcessTime   string  `json:"processTime"`
	CreateTime    string  `json:"createTime"`
}

// ---- 内存存储（MVP 阶段不连 DB）----

type withdrawalStore struct {
	mu   sync.RWMutex
	list []Withdrawal
	seq  int64
}

var globalWithdrawalStore = &withdrawalStore{
	list: []Withdrawal{
		{
			Id:            1,
			WithdrawalNo:  "WD20260701001",
			ApplicantType: SettleTypeTemple,
			ApplicantId:   "T001",
			Amount:        2000,
			BankCard:      "6222021234567890",
			Status:        WithdrawalPending,
			CreateTime:    "2026-07-01 09:00:00",
		},
		{
			Id:            2,
			WithdrawalNo:  "WD20260628002",
			ApplicantType: SettleTypeMaster,
			ApplicantId:   "M001",
			Amount:        1500,
			BankCard:      "6222020987654321",
			Status:        WithdrawalSuccess,
			AuditTime:     "2026-06-28 14:00:00",
			ProcessTime:   "2026-06-29 10:00:00",
			CreateTime:    "2026-06-28 10:00:00",
		},
	},
	seq: 2,
}

// ListWithdrawals 查询提现列表
func ListWithdrawals(applicantType, status string, page, size int) ([]Withdrawal, int64) {
	globalWithdrawalStore.mu.RLock()
	defer globalWithdrawalStore.mu.RUnlock()

	filtered := make([]Withdrawal, 0, len(globalWithdrawalStore.list))
	for _, w := range globalWithdrawalStore.list {
		if applicantType != "" && w.ApplicantType != applicantType {
			continue
		}
		if status != "" && w.Status != status {
			continue
		}
		filtered = append(filtered, w)
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

// FindWithdrawalByID 按ID查询提现
func FindWithdrawalByID(id int64) (Withdrawal, bool) {
	globalWithdrawalStore.mu.RLock()
	defer globalWithdrawalStore.mu.RUnlock()
	for _, w := range globalWithdrawalStore.list {
		if w.Id == id {
			return w, true
		}
	}
	return Withdrawal{}, false
}

// UpdateWithdrawalStatus 更新提现状态
func UpdateWithdrawalStatus(id int64, status, auditTime, processTime string) bool {
	globalWithdrawalStore.mu.Lock()
	defer globalWithdrawalStore.mu.Unlock()
	for i := range globalWithdrawalStore.list {
		if globalWithdrawalStore.list[i].Id == id {
			globalWithdrawalStore.list[i].Status = status
			if auditTime != "" {
				globalWithdrawalStore.list[i].AuditTime = auditTime
			}
			if processTime != "" {
				globalWithdrawalStore.list[i].ProcessTime = processTime
			}
			return true
		}
	}
	return false
}

// CountWithdrawalByStatus 按状态计数提现单
func CountWithdrawalByStatus(status string) int64 {
	globalWithdrawalStore.mu.RLock()
	defer globalWithdrawalStore.mu.RUnlock()
	var count int64
	for _, w := range globalWithdrawalStore.list {
		if w.Status == status {
			count++
		}
	}
	return count
}

// ApplyWithdrawal 法师/寺院/商家提交提现申请，返回新建的提现单
func ApplyWithdrawal(applicantType, applicantId string, amount float64, bankCard string) Withdrawal {
	globalWithdrawalStore.mu.Lock()
	defer globalWithdrawalStore.mu.Unlock()

	globalWithdrawalStore.seq++
	now := time.Now().Format("2006-01-02 15:04:05")
	w := Withdrawal{
		Id:            globalWithdrawalStore.seq,
		WithdrawalNo:  fmt.Sprintf("WD%s%03d", now[:8], globalWithdrawalStore.seq),
		ApplicantType: applicantType,
		ApplicantId:   applicantId,
		Amount:        amount,
		BankCard:      bankCard,
		Status:        WithdrawalPending,
		CreateTime:    now,
	}
	globalWithdrawalStore.list = append(globalWithdrawalStore.list, w)
	return w
}
