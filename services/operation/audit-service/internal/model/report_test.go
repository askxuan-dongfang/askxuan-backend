package model

import "testing"

// ===== 举报状态机测试（运营审核服务） =====

func TestCanTransitReport(t *testing.T) {
	tests := []struct {
		name   string
		from   string
		to     string
		expect bool
	}{
		// 合法流转
		{"pending→handled", ReportStatusPending, ReportStatusHandled, true},
		{"pending→rejected", ReportStatusPending, ReportStatusRejected, true},

		// 非法流转
		{"handled→pending(已处理不能回退)", ReportStatusHandled, ReportStatusPending, false},
		{"handled→rejected(已处理不能驳回)", ReportStatusHandled, ReportStatusRejected, false},
		{"rejected→pending(已驳回不能回退)", ReportStatusRejected, ReportStatusPending, false},
		{"rejected→handled(已驳回不能处理)", ReportStatusRejected, ReportStatusHandled, false},
		{"相同状态", ReportStatusPending, ReportStatusPending, false},
		{"未知源状态", "unknown", ReportStatusHandled, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanTransitReport(tt.from, tt.to)
			if got != tt.expect {
				t.Errorf("CanTransitReport(%q→%q) 期望 %v, 实际 %v", tt.from, tt.to, tt.expect, got)
			}
		})
	}
}
