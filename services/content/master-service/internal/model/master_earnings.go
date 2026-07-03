package model

import (
	"sync"
	"time"
)

// ============ 法师收益 - 内存存储（MVP-3 阶段） ============

// 收益结算状态常量
const (
	EarningsSettlePending  = "pending"  // 待结算
	EarningsSettleSettled  = "settled"  // 已结算
	EarningsSettleWithdrew = "withdrew" // 已提现
)

// 服务类型常量
const (
	EarningsServiceBooking  = "booking"        // 预约法事
	EarningsServiceBlessing = "diy_blessing"   // DIY加持
	EarningsServiceConsult  = "consult"        // 咨询
)

// EarningsDetail 收益明细
type EarningsDetail struct {
	Id           int64   `json:"id"`
	MasterCode   string  `json:"masterCode"`
	Date         string  `json:"date"`
	ServiceType  string  `json:"serviceType"`
	UserName     string  `json:"userName"`
	Amount       float64 `json:"amount"`
	SettleStatus string  `json:"settleStatus"`
	CreateTime   string  `json:"createTime"`
}

// earningsStore 内存存储
type earningsStore struct {
	mu   sync.RWMutex
	list []EarningsDetail
	seq  int64
}

// globalEarningsStore 全局收益存储，预置 mock 数据
var globalEarningsStore = &earningsStore{
	list: []EarningsDetail{
		{
			Id:           1,
			MasterCode:   "M001",
			Date:         "2026-07-01",
			ServiceType:  EarningsServiceBooking,
			UserName:     "张三",
			Amount:       500,
			SettleStatus: EarningsSettleSettled,
			CreateTime:   "2026-07-01 10:00:00",
		},
		{
			Id:           2,
			MasterCode:   "M001",
			Date:         "2026-07-01",
			ServiceType:  EarningsServiceBlessing,
			UserName:     "李四",
			Amount:       300,
			SettleStatus: EarningsSettlePending,
			CreateTime:   "2026-07-01 14:00:00",
		},
		{
			Id:           3,
			MasterCode:   "M001",
			Date:         "2026-06-28",
			ServiceType:  EarningsServiceConsult,
			UserName:     "王五",
			Amount:       200,
			SettleStatus: EarningsSettleWithdrew,
			CreateTime:   "2026-06-28 09:30:00",
		},
		{
			Id:           4,
			MasterCode:   "M001",
			Date:         "2026-06-25",
			ServiceType:  EarningsServiceBooking,
			UserName:     "赵六",
			Amount:       800,
			SettleStatus: EarningsSettleSettled,
			CreateTime:   "2026-06-25 11:00:00",
		},
		{
			Id:           5,
			MasterCode:   "M001",
			Date:         "2026-06-20",
			ServiceType:  EarningsServiceBlessing,
			UserName:     "孙七",
			Amount:       450,
			SettleStatus: EarningsSettleSettled,
			CreateTime:   "2026-06-20 16:00:00",
		},
	},
	seq: 5,
}

// ListEarnings 查询法师收益明细列表，支持按 serviceType 筛选 + 分页
func ListEarnings(masterCode, serviceType string, page, size int) ([]EarningsDetail, int64) {
	globalEarningsStore.mu.RLock()
	defer globalEarningsStore.mu.RUnlock()

	filtered := make([]EarningsDetail, 0, len(globalEarningsStore.list))
	for _, e := range globalEarningsStore.list {
		if e.MasterCode != masterCode {
			continue
		}
		if serviceType != "" && e.ServiceType != serviceType {
			continue
		}
		filtered = append(filtered, e)
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

// EarningsSummary 收益汇总
type EarningsSummary struct {
	MonthIncome  float64            `json:"monthIncome"`
	TotalIncome  float64            `json:"totalIncome"`
	Withdrawable float64            `json:"withdrawable"`
	Trend        []EarningsTrendRow `json:"trend"`
}

// EarningsTrendRow 收益趋势行
type EarningsTrendRow struct {
	Month  string  `json:"month"`
	Amount float64 `json:"amount"`
}

// GetEarningsSummary 汇总法师收益：本月收入、总收入、可提现、近6月趋势
func GetEarningsSummary(masterCode string) EarningsSummary {
	globalEarningsStore.mu.RLock()
	defer globalEarningsStore.mu.RUnlock()

	now := time.Now()
	currentMonth := now.Format("2006-01")

	var monthIncome, totalIncome, withdrawable float64
	monthTrend := map[string]float64{}

	for _, e := range globalEarningsStore.list {
		if e.MasterCode != masterCode {
			continue
		}
		totalIncome += e.Amount
		// 月份收入：取 date 的 yyyy-MM 部分
		if len(e.Date) >= 7 && e.Date[:7] == currentMonth {
			monthIncome += e.Amount
		}
		// 可提现：已结算且未提现
		if e.SettleStatus == EarningsSettleSettled {
			withdrawable += e.Amount
		}
		// 趋势：按 yyyy-MM 聚合
		if len(e.Date) >= 7 {
			monthTrend[e.Date[:7]] += e.Amount
		}
	}

	// 生成近 6 个月趋势（含当月）
	trend := make([]EarningsTrendRow, 0, 6)
	for i := 5; i >= 0; i-- {
		m := now.AddDate(0, -i, 0).Format("2006-01")
		trend = append(trend, EarningsTrendRow{
			Month:  m,
			Amount: monthTrend[m],
		})
	}

	return EarningsSummary{
		MonthIncome:  monthIncome,
		TotalIncome:  totalIncome,
		Withdrawable: withdrawable,
		Trend:        trend,
	}
}
