package model

import "testing"

// ===== DIY订单状态机测试 =====

func TestCanDiyTransit(t *testing.T) {
	tests := []struct {
		name   string
		from   string
		to     string
		expect bool
	}{
		// 合法流转
		{"pending_review→in_making", DiyStatusPendingReview, DiyStatusInMaking, true},
		{"pending_review→cancelled", DiyStatusPendingReview, DiyStatusCancelled, true},
		{"in_making→awaiting_blessing", DiyStatusInMaking, DiyStatusAwaitingBlessing, true},
		{"in_making→awaiting_shipment", DiyStatusInMaking, DiyStatusAwaitingShipment, true},
		{"in_making→cancelled", DiyStatusInMaking, DiyStatusCancelled, true},
		{"awaiting_blessing→blessing_in_progress", DiyStatusAwaitingBlessing, DiyStatusBlessingInProgress, true},
		{"blessing_in_progress→blessing_completed", DiyStatusBlessingInProgress, DiyStatusBlessingCompleted, true},
		{"blessing_completed→awaiting_shipment", DiyStatusBlessingCompleted, DiyStatusAwaitingShipment, true},
		{"awaiting_shipment→shipped", DiyStatusAwaitingShipment, DiyStatusShipped, true},
		{"shipped→completed", DiyStatusShipped, DiyStatusCompleted, true},
		{"shipped→in_return", DiyStatusShipped, DiyStatusInReturn, true},
		{"in_return→completed", DiyStatusInReturn, DiyStatusCompleted, true},
		{"in_return→shipped", DiyStatusInReturn, DiyStatusShipped, true},

		// 非法流转
		{"pending_review→awaiting_blessing(跳过制作)", DiyStatusPendingReview, DiyStatusAwaitingBlessing, false},
		{"pending_review→shipped(跳过制作和发货)", DiyStatusPendingReview, DiyStatusShipped, false},
		{"in_making→shipped(跳过待发货)", DiyStatusInMaking, DiyStatusShipped, false},
		{"awaiting_blessing→awaiting_shipment(跳过加持)", DiyStatusAwaitingBlessing, DiyStatusAwaitingShipment, false},
		{"blessing_in_progress→awaiting_shipment(跳过加持完成)", DiyStatusBlessingInProgress, DiyStatusAwaitingShipment, false},
		{"shipped→awaiting_shipment(已发货不能回退)", DiyStatusShipped, DiyStatusAwaitingShipment, false},
		{"completed→shipped(已完成不能回退)", DiyStatusCompleted, DiyStatusShipped, false},
		{"cancelled→in_making(已取消不能恢复)", DiyStatusCancelled, DiyStatusInMaking, false},
		{"相同状态", DiyStatusPendingReview, DiyStatusPendingReview, false},
		{"未知源状态", "unknown", DiyStatusInMaking, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanDiyTransit(tt.from, tt.to)
			if got != tt.expect {
				t.Errorf("CanDiyTransit(%q→%q) 期望 %v, 实际 %v", tt.from, tt.to, tt.expect, got)
			}
		})
	}
}

func TestIsDiyTerminalStatus(t *testing.T) {
	// completed 和 cancelled 是终态
	terminal := []string{DiyStatusCompleted, DiyStatusCancelled}
	for _, s := range terminal {
		if !IsDiyTerminalStatus(s) {
			t.Errorf("状态 %q 应为终态", s)
		}
	}

	// 其他状态不是终态
	nonTerminal := []string{
		DiyStatusPendingReview,
		DiyStatusInMaking,
		DiyStatusAwaitingBlessing,
		DiyStatusBlessingInProgress,
		DiyStatusBlessingCompleted,
		DiyStatusAwaitingShipment,
		DiyStatusShipped,
		DiyStatusInReturn,
		"unknown",
	}
	for _, s := range nonTerminal {
		if IsDiyTerminalStatus(s) {
			t.Errorf("状态 %q 不应为终态", s)
		}
	}
}
