package model

import "testing"

// ===== 预约状态机测试 =====

func TestCanTransit(t *testing.T) {
	tests := []struct {
		name   string
		from   string
		to     string
		expect bool
	}{
		// 合法流转
		{"pending→confirmed", StatusPending, StatusConfirmed, true},
		{"pending→cancelled", StatusPending, StatusCancelled, true},
		{"confirmed→in_progress", StatusConfirmed, StatusInProgress, true},
		{"confirmed→cancelled", StatusConfirmed, StatusCancelled, true},
		{"in_progress→completed", StatusInProgress, StatusCompleted, true},
		{"in_progress→cancelled", StatusInProgress, StatusCancelled, true},
		{"completed→reviewed", StatusCompleted, StatusReviewed, true},

		// 非法流转（不能跳跃）
		{"pending→in_progress(跳过确认)", StatusPending, StatusInProgress, false},
		{"pending→completed(跳过确认和进行)", StatusPending, StatusCompleted, false},
		{"confirmed→reviewed(跳过完成)", StatusConfirmed, StatusReviewed, false},
		{"cancelled→confirmed(已取消不能恢复)", StatusCancelled, StatusConfirmed, false},
		{"reviewed→completed(已评价不能回退)", StatusReviewed, StatusCompleted, false},
		{"相同状态", StatusPending, StatusPending, false},
		{"未知源状态", "unknown", StatusConfirmed, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanTransit(tt.from, tt.to)
			if got != tt.expect {
				t.Errorf("CanTransit(%q→%q) 期望 %v, 实际 %v", tt.from, tt.to, tt.expect, got)
			}
		})
	}
}

func TestIsTerminalStatus(t *testing.T) {
	// reviewed 和 cancelled 是终态
	if !IsTerminalStatus(StatusReviewed) {
		t.Error("reviewed 应为终态")
	}
	if !IsTerminalStatus(StatusCancelled) {
		t.Error("cancelled 应为终态")
	}

	// 其他状态不是终态
	nonTerminal := []string{StatusPending, StatusConfirmed, StatusInProgress, StatusCompleted}
	for _, s := range nonTerminal {
		if IsTerminalStatus(s) {
			t.Errorf("状态 %q 不应为终态", s)
		}
	}
}
