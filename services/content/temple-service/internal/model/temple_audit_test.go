package model

import "testing"

// ===== 寺院入驻审核状态机测试 =====

func TestCanTransitTempleAudit(t *testing.T) {
	tests := []struct {
		name   string
		from   string
		to     string
		expect bool
	}{
		// 合法流转
		{"pending→first_pass", TempleAuditStatusPending, TempleAuditStatusFirstPass, true},
		{"pending→rejected", TempleAuditStatusPending, TempleAuditStatusRejected, true},
		{"first_pass→final_pass", TempleAuditStatusFirstPass, TempleAuditStatusFinalPass, true},
		{"first_pass→rejected", TempleAuditStatusFirstPass, TempleAuditStatusRejected, true},
		{"rejected→pending(修改后重新提交)", TempleAuditStatusRejected, TempleAuditStatusPending, true},

		// 非法流转
		{"pending→final_pass(跳过初审)", TempleAuditStatusPending, TempleAuditStatusFinalPass, false},
		{"first_pass→pending(初审通过不能回退)", TempleAuditStatusFirstPass, TempleAuditStatusPending, false},
		{"final_pass→pending(终审通过不能回退)", TempleAuditStatusFinalPass, TempleAuditStatusPending, false},
		{"final_pass→rejected(终审通过不能驳回)", TempleAuditStatusFinalPass, TempleAuditStatusRejected, false},
		{"rejected→first_pass(驳回后只能回到待审核)", TempleAuditStatusRejected, TempleAuditStatusFirstPass, false},
		{"相同状态", TempleAuditStatusPending, TempleAuditStatusPending, false},
		{"未知源状态", "unknown", TempleAuditStatusFirstPass, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanTransitTempleAudit(tt.from, tt.to)
			if got != tt.expect {
				t.Errorf("CanTransitTempleAudit(%q→%q) 期望 %v, 实际 %v", tt.from, tt.to, tt.expect, got)
			}
		})
	}
}
