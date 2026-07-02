package model

import "testing"

// ===== 支付状态机测试 =====

func TestCanPaymentTransit(t *testing.T) {
	tests := []struct {
		name   string
		from   string
		to     string
		expect bool
	}{
		// 合法流转
		{"pending→success", PaymentStatusPending, PaymentStatusSuccess, true},
		{"pending→failed", PaymentStatusPending, PaymentStatusFailed, true},
		{"pending→closed", PaymentStatusPending, PaymentStatusClosed, true},
		{"success→refunding", PaymentStatusSuccess, PaymentStatusRefunding, true},
		{"refunding→refunded", PaymentStatusRefunding, PaymentStatusRefunded, true},
		{"refunding→success(退款失败恢复)", PaymentStatusRefunding, PaymentStatusSuccess, true},

		// 非法流转
		{"pending→refunded(跳过支付)", PaymentStatusPending, PaymentStatusRefunded, false},
		{"failed→success(失败不能恢复)", PaymentStatusFailed, PaymentStatusSuccess, false},
		{"closed→success(关闭不能恢复)", PaymentStatusClosed, PaymentStatusSuccess, false},
		{"refunded→success(已退款不能恢复)", PaymentStatusRefunded, PaymentStatusSuccess, false},
		{"success→pending(不能回退)", PaymentStatusSuccess, PaymentStatusPending, false},
		{"相同状态", PaymentStatusPending, PaymentStatusPending, false},
		{"未知源状态", "unknown", PaymentStatusSuccess, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanPaymentTransit(tt.from, tt.to)
			if got != tt.expect {
				t.Errorf("CanPaymentTransit(%q→%q) 期望 %v, 实际 %v", tt.from, tt.to, tt.expect, got)
			}
		})
	}
}

func TestIsPaymentTerminalStatus(t *testing.T) {
	terminal := []string{PaymentStatusFailed, PaymentStatusClosed, PaymentStatusRefunded, PaymentStatusSuccess}
	nonTerminal := []string{PaymentStatusPending, PaymentStatusRefunding, "unknown"}

	for _, s := range terminal {
		if !IsPaymentTerminalStatus(s) {
			t.Errorf("状态 %q 应为终态", s)
		}
	}
	for _, s := range nonTerminal {
		if IsPaymentTerminalStatus(s) {
			t.Errorf("状态 %q 不应为终态", s)
		}
	}
}
