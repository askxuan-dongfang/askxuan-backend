package model

import "testing"

// ===== 结算单状态机测试 =====

func TestCanTransitSettlement(t *testing.T) {
	tests := []struct {
		name   string
		from   string
		to     string
		expect bool
	}{
		// 合法流转
		{"pending→confirmed", SettlementPending, SettlementConfirmed, true},
		{"confirmed→paid", SettlementConfirmed, SettlementPaid, true},

		// 非法流转
		{"pending→paid(跳过确认)", SettlementPending, SettlementPaid, false},
		{"confirmed→pending(已确认不能回退)", SettlementConfirmed, SettlementPending, false},
		{"paid→confirmed(已打款不能回退)", SettlementPaid, SettlementConfirmed, false},
		{"paid→pending(已打款不能回退)", SettlementPaid, SettlementPending, false},
		{"相同状态", SettlementPending, SettlementPending, false},
		{"未知源状态", "unknown", SettlementConfirmed, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanTransitSettlement(tt.from, tt.to)
			if got != tt.expect {
				t.Errorf("CanTransitSettlement(%q→%q) 期望 %v, 实际 %v", tt.from, tt.to, tt.expect, got)
			}
		})
	}
}
