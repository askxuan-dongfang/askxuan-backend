package model

import "testing"

// ===== 退换货状态机测试 =====

func TestCanReturnTransit(t *testing.T) {
	tests := []struct {
		name   string
		from   string
		to     string
		expect bool
	}{
		// 合法流转
		{"pending_review→approved", ReturnStatusPendingReview, ReturnStatusApproved, true},
		{"pending_review→rejected", ReturnStatusPendingReview, ReturnStatusRejected, true},
		{"approved→return_shipping", ReturnStatusApproved, ReturnStatusReturnShipping, true},
		{"return_shipping→return_received", ReturnStatusReturnShipping, ReturnStatusReturnReceived, true},
		{"return_received→refunding", ReturnStatusReturnReceived, ReturnStatusRefunding, true},
		{"refunding→completed", ReturnStatusRefunding, ReturnStatusCompleted, true},

		// 非法流转
		{"pending_review→return_shipping(跳过审核)", ReturnStatusPendingReview, ReturnStatusReturnShipping, false},
		{"pending_review→completed(跳过流程)", ReturnStatusPendingReview, ReturnStatusCompleted, false},
		{"approved→refunding(跳过退货运输和收货)", ReturnStatusApproved, ReturnStatusRefunding, false},
		{"approved→rejected(已同意不能驳回)", ReturnStatusApproved, ReturnStatusRejected, false},
		{"rejected→approved(已拒绝不能恢复)", ReturnStatusRejected, ReturnStatusApproved, false},
		{"completed→refunding(已完成不能回退)", ReturnStatusCompleted, ReturnStatusRefunding, false},
		{"相同状态", ReturnStatusPendingReview, ReturnStatusPendingReview, false},
		{"未知源状态", "unknown", ReturnStatusApproved, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanReturnTransit(tt.from, tt.to)
			if got != tt.expect {
				t.Errorf("CanReturnTransit(%q→%q) 期望 %v, 实际 %v", tt.from, tt.to, tt.expect, got)
			}
		})
	}
}
