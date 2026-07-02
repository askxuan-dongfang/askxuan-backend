package model

import (
	"sync"
)

// 结算单状态常量
const (
	SettlementPending   = "pending"   // 待确认
	SettlementConfirmed = "confirmed" // 已确认
	SettlementPaid      = "paid"      // 已打款
)

// 结算类型（结算单与提现申请共用）
const (
	SettleTypeTemple = "temple"
	SettleTypeMaster = "master"
	SettleTypeShop   = "shop"
)

// settlementTransitions 结算单状态机合法流转
var settlementTransitions = map[string]map[string]bool{
	SettlementPending: {
		SettlementConfirmed: true,
	},
	SettlementConfirmed: {
		SettlementPaid: true,
	},
}

// CanTransitSettlement 校验结算单状态流转是否合法
func CanTransitSettlement(from, to string) bool {
	if from == to {
		return false
	}
	allowed, ok := settlementTransitions[from]
	if !ok {
		return false
	}
	return allowed[to]
}

// Settlement 结算单结构体
type Settlement struct {
	Id               int64   `json:"id"`
	SettlementNo     string  `json:"settlementNo"`
	SettleType       string  `json:"settleType"`
	TargetId         string  `json:"targetId"`
	TargetName       string  `json:"targetName"`
	PeriodStart      string  `json:"periodStart"`
	PeriodEnd        string  `json:"periodEnd"`
	OrderCount       int     `json:"orderCount"`
	TotalAmount      float64 `json:"totalAmount"`
	CommissionRate   float64 `json:"commissionRate"`
	CommissionAmount float64 `json:"commissionAmount"`
	SettleAmount     float64 `json:"settleAmount"`
	Status           string  `json:"status"`
	CreateTime       string  `json:"createTime"`
}

// ---- 内存存储（MVP 阶段不连 DB）----

type settlementStore struct {
	mu  sync.RWMutex
	list []Settlement
	seq  int64
}

var globalSettlementStore = &settlementStore{
	list: []Settlement{
		{
			Id:               1,
			SettlementNo:     "SET2026060001",
			SettleType:       SettleTypeTemple,
			TargetId:         "T001",
			TargetName:       "灵隐寺",
			PeriodStart:      "2026-06-01 00:00:00",
			PeriodEnd:        "2026-06-30 23:59:59",
			OrderCount:       15,
			TotalAmount:      3500,
			CommissionRate:   0.15,
			CommissionAmount: 525,
			SettleAmount:     2975,
			Status:           SettlementConfirmed,
			CreateTime:       "2026-07-01 02:00:00",
		},
		{
			Id:               2,
			SettlementNo:     "SET2026060002",
			SettleType:       SettleTypeMaster,
			TargetId:         "M001",
			TargetName:       "智海法师",
			PeriodStart:      "2026-06-01 00:00:00",
			PeriodEnd:        "2026-06-30 23:59:59",
			OrderCount:       12,
			TotalAmount:      2800,
			CommissionRate:   0.15,
			CommissionAmount: 420,
			SettleAmount:     2380,
			Status:           SettlementPending,
			CreateTime:       "2026-07-01 02:00:00",
		},
		{
			Id:               3,
			SettlementNo:     "SET2026060003",
			SettleType:       SettleTypeShop,
			TargetId:         "SHOP001",
			TargetName:       "东方商城",
			PeriodStart:      "2026-06-01 00:00:00",
			PeriodEnd:        "2026-06-30 23:59:59",
			OrderCount:       86,
			TotalAmount:      25600,
			CommissionRate:   0.10,
			CommissionAmount: 2560,
			SettleAmount:     23040,
			Status:           SettlementPaid,
			CreateTime:       "2026-07-01 02:00:00",
		},
	},
	seq: 3,
}

// ListSettlements 查询结算列表
func ListSettlements(settleType, status string, page, size int) ([]Settlement, int64) {
	globalSettlementStore.mu.RLock()
	defer globalSettlementStore.mu.RUnlock()

	filtered := make([]Settlement, 0, len(globalSettlementStore.list))
	for _, s := range globalSettlementStore.list {
		if settleType != "" && s.SettleType != settleType {
			continue
		}
		if status != "" && s.Status != status {
			continue
		}
		filtered = append(filtered, s)
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

// FindSettlementByID 按ID查询结算单
func FindSettlementByID(id int64) (Settlement, bool) {
	globalSettlementStore.mu.RLock()
	defer globalSettlementStore.mu.RUnlock()
	for _, s := range globalSettlementStore.list {
		if s.Id == id {
			return s, true
		}
	}
	return Settlement{}, false
}
