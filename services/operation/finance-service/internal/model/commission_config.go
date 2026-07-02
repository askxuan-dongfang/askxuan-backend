package model

import (
	"sync"
)

// 业务类型（抽成配置使用）
const (
	BizTypeBooking     = "booking"
	BizTypeDiyBlessing = "diy_blessing"
	BizTypeDiyMaterial = "diy_material"
	BizTypeShopOrder   = "shop_order"
)

// CommissionConfig 抽成配置结构体
type CommissionConfig struct {
	Id          int64   `json:"id"`
	BizType     string  `json:"bizType"`
	Rate        float64 `json:"rate"`
	Description string  `json:"description"`
	UpdateTime  string  `json:"updateTime"`
}

// ---- 内存存储（MVP 阶段不连 DB）----

type commissionConfigStore struct {
	mu   sync.RWMutex
	list []CommissionConfig
}

var globalCommissionConfigStore = &commissionConfigStore{
	list: []CommissionConfig{
		{Id: 1, BizType: BizTypeBooking, Rate: 0.15, Description: "预约服务平台抽成15%", UpdateTime: "2026-07-01 00:00:00"},
		{Id: 2, BizType: BizTypeDiyBlessing, Rate: 0.15, Description: "DIY加持费平台抽成15%", UpdateTime: "2026-07-01 00:00:00"},
		{Id: 3, BizType: BizTypeDiyMaterial, Rate: 0.10, Description: "DIY材料费平台抽成10%", UpdateTime: "2026-07-01 00:00:00"},
		{Id: 4, BizType: BizTypeShopOrder, Rate: 0.10, Description: "商城订单平台抽成10%", UpdateTime: "2026-07-01 00:00:00"},
	},
}

// ListCommissionConfigs 查询抽成配置列表
func ListCommissionConfigs(bizType string) []CommissionConfig {
	globalCommissionConfigStore.mu.RLock()
	defer globalCommissionConfigStore.mu.RUnlock()

	result := make([]CommissionConfig, 0, len(globalCommissionConfigStore.list))
	for _, c := range globalCommissionConfigStore.list {
		if bizType != "" && c.BizType != bizType {
			continue
		}
		result = append(result, c)
	}
	return result
}
