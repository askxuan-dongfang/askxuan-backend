package model

import "testing"

// ===== 加持任务状态机测试（法师侧） =====

func TestCanTransitBlessingTask(t *testing.T) {
	tests := []struct {
		name   string
		from   string
		to     string
		expect bool
	}{
		// 合法流转
		{"dispatched→assigned", BlessingTaskStatusDispatched, BlessingTaskStatusAssigned, true},
		{"assigned→accepted", BlessingTaskStatusAssigned, BlessingTaskStatusAccepted, true},
		{"assigned→rejected", BlessingTaskStatusAssigned, BlessingTaskStatusRejected, true},
		{"rejected→assigned(重新分配)", BlessingTaskStatusRejected, BlessingTaskStatusAssigned, true},
		{"accepted→in_progress", BlessingTaskStatusAccepted, BlessingTaskStatusInProgress, true},
		{"in_progress→completed", BlessingTaskStatusInProgress, BlessingTaskStatusCompleted, true},

		// 非法流转
		{"dispatched→accepted(跳过分配)", BlessingTaskStatusDispatched, BlessingTaskStatusAccepted, false},
		{"dispatched→completed(跳过流程)", BlessingTaskStatusDispatched, BlessingTaskStatusCompleted, false},
		{"assigned→in_progress(跳过接受)", BlessingTaskStatusAssigned, BlessingTaskStatusInProgress, false},
		{"accepted→completed(跳过加持中)", BlessingTaskStatusAccepted, BlessingTaskStatusCompleted, false},
		{"completed→in_progress(已完成不能回退)", BlessingTaskStatusCompleted, BlessingTaskStatusInProgress, false},
		{"rejected→accepted(拒绝后必须重新分配)", BlessingTaskStatusRejected, BlessingTaskStatusAccepted, false},
		{"相同状态", BlessingTaskStatusDispatched, BlessingTaskStatusDispatched, false},
		{"未知源状态", "unknown", BlessingTaskStatusAssigned, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanTransitBlessingTask(tt.from, tt.to)
			if got != tt.expect {
				t.Errorf("CanTransitBlessingTask(%q→%q) 期望 %v, 实际 %v", tt.from, tt.to, tt.expect, got)
			}
		})
	}
}
