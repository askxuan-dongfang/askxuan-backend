package model

import "testing"

// ===== 法师资质审核状态机测试 =====

func TestCanTransitMasterAudit(t *testing.T) {
	tests := []struct {
		name   string
		from   string
		to     string
		expect bool
	}{
		// 合法流转
		{"pending→pass", MasterAuditStatusPending, MasterAuditStatusPass, true},
		{"pending→rejected", MasterAuditStatusPending, MasterAuditStatusRejected, true},
		{"rejected→pending(修改后重新提交)", MasterAuditStatusRejected, MasterAuditStatusPending, true},

		// 非法流转
		{"pending→pending(相同状态)", MasterAuditStatusPending, MasterAuditStatusPending, false},
		{"pass→pending(审核通过不能回退)", MasterAuditStatusPass, MasterAuditStatusPending, false},
		{"pass→rejected(审核通过不能驳回)", MasterAuditStatusPass, MasterAuditStatusRejected, false},
		{"rejected→pass(驳回后必须先回到待审核)", MasterAuditStatusRejected, MasterAuditStatusPass, false},
		{"相同状态", MasterAuditStatusPending, MasterAuditStatusPending, false},
		{"未知源状态", "unknown", MasterAuditStatusPass, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanTransitMasterAudit(tt.from, tt.to)
			if got != tt.expect {
				t.Errorf("CanTransitMasterAudit(%q→%q) 期望 %v, 实际 %v", tt.from, tt.to, tt.expect, got)
			}
		})
	}
}
