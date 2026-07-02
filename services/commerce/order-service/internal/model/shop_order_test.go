package model

import "testing"

// ===== 商城订单状态机测试 =====

func TestCanOrderTransit(t *testing.T) {
	tests := []struct {
		name   string
		from   string
		to     string
		expect bool
	}{
		// 合法流转
		{"pending_payment→paid", OrderStatusPendingPayment, OrderStatusPaid, true},
		{"pending_payment→cancelled", OrderStatusPendingPayment, OrderStatusCancelled, true},
		{"paid→shipped", OrderStatusPaid, OrderStatusShipped, true},
		{"paid→cancelled", OrderStatusPaid, OrderStatusCancelled, true},
		{"shipped→completed", OrderStatusShipped, OrderStatusCompleted, true},
		{"shipped→in_return", OrderStatusShipped, OrderStatusInReturn, true},
		{"in_return→completed", OrderStatusInReturn, OrderStatusCompleted, true},
		{"in_return→shipped", OrderStatusInReturn, OrderStatusShipped, true},

		// 非法流转
		{"pending_payment→shipped(跳过支付)", OrderStatusPendingPayment, OrderStatusShipped, false},
		{"pending_payment→completed(跳过支付和发货)", OrderStatusPendingPayment, OrderStatusCompleted, false},
		{"paid→completed(跳过发货)", OrderStatusPaid, OrderStatusCompleted, false},
		{"paid→in_return(已支付不能直接售后)", OrderStatusPaid, OrderStatusInReturn, false},
		{"completed→shipped(已完成不能回退)", OrderStatusCompleted, OrderStatusShipped, false},
		{"cancelled→paid(已取消不能恢复)", OrderStatusCancelled, OrderStatusPaid, false},
		{"相同状态", OrderStatusPendingPayment, OrderStatusPendingPayment, false},
		{"未知源状态", "unknown", OrderStatusPaid, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanOrderTransit(tt.from, tt.to)
			if got != tt.expect {
				t.Errorf("CanOrderTransit(%q→%q) 期望 %v, 实际 %v", tt.from, tt.to, tt.expect, got)
			}
		})
	}
}

func TestIsOrderTerminalStatus(t *testing.T) {
	// completed 和 cancelled 是终态
	terminal := []string{OrderStatusCompleted, OrderStatusCancelled}
	for _, s := range terminal {
		if !IsOrderTerminalStatus(s) {
			t.Errorf("状态 %q 应为终态", s)
		}
	}

	// 其他状态不是终态
	nonTerminal := []string{OrderStatusPendingPayment, OrderStatusPaid, OrderStatusShipped, OrderStatusInReturn, "unknown"}
	for _, s := range nonTerminal {
		if IsOrderTerminalStatus(s) {
			t.Errorf("状态 %q 不应为终态", s)
		}
	}
}
