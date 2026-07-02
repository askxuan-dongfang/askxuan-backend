package model

import "testing"

// ===== 提现状态机测试 =====

func TestCanTransitWithdrawal(t *testing.T) {
	tests := []struct {
		name   string
		from   string
		to     string
		expect bool
	}{
		// 合法流转
		{"pending→approved", WithdrawalPending, WithdrawalApproved, true},
		{"pending→rejected", WithdrawalPending, WithdrawalRejected, true},
		{"approved→processing", WithdrawalApproved, WithdrawalProcessing, true},
		{"processing→success", WithdrawalProcessing, WithdrawalSuccess, true},
		{"processing→failed", WithdrawalProcessing, WithdrawalFailed, true},
		{"failed→processing(打款失败重试)", WithdrawalFailed, WithdrawalProcessing, true},

		// 非法流转
		{"pending→processing(跳过审核)", WithdrawalPending, WithdrawalProcessing, false},
		{"pending→success(跳过审核和打款)", WithdrawalPending, WithdrawalSuccess, false},
		{"approved→success(跳过打款中)", WithdrawalApproved, WithdrawalSuccess, false},
		{"approved→rejected(已审核不能驳回)", WithdrawalApproved, WithdrawalRejected, false},
		{"success→processing(打款成功不能回退)", WithdrawalSuccess, WithdrawalProcessing, false},
		{"success→failed(打款成功不能变为失败)", WithdrawalSuccess, WithdrawalFailed, false},
		{"rejected→approved(已拒绝不能恢复)", WithdrawalRejected, WithdrawalApproved, false},
		{"failed→success(失败必须重新打款)", WithdrawalFailed, WithdrawalSuccess, false},
		{"相同状态", WithdrawalPending, WithdrawalPending, false},
		{"未知源状态", "unknown", WithdrawalApproved, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanTransitWithdrawal(tt.from, tt.to)
			if got != tt.expect {
				t.Errorf("CanTransitWithdrawal(%q→%q) 期望 %v, 实际 %v", tt.from, tt.to, tt.expect, got)
			}
		})
	}
}
