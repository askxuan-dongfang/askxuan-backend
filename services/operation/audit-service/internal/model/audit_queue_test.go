package model

import "testing"

// ===== 审核队列状态机测试 =====

func TestCanTransitAudit(t *testing.T) {
	tests := []struct {
		name   string
		from   string
		to     string
		expect bool
	}{
		// 合法流转
		{"pending→approved(设计通过)", AuditStatusPending, AuditStatusApproved, true},
		{"pending→rejected(驳回)", AuditStatusPending, AuditStatusRejected, true},
		{"pending→first_pass(寺院初审通过)", AuditStatusPending, AuditStatusFirstPass, true},
		{"pending→verified(法师认证通过)", AuditStatusPending, AuditStatusVerified, true},
		{"first_pass→final_pass(寺院终审通过)", AuditStatusFirstPass, AuditStatusFinalPass, true},
		{"first_pass→rejected(终审驳回)", AuditStatusFirstPass, AuditStatusRejected, true},
		{"rejected→pending(修改后重新提交)", AuditStatusRejected, AuditStatusPending, true},

		// 非法流转
		{"pending→final_pass(跳过初审)", AuditStatusPending, AuditStatusFinalPass, false},
		{"approved→pending(审核通过不能回退)", AuditStatusApproved, AuditStatusPending, false},
		{"approved→rejected(审核通过不能驳回)", AuditStatusApproved, AuditStatusRejected, false},
		{"final_pass→pending(终审通过不能回退)", AuditStatusFinalPass, AuditStatusPending, false},
		{"verified→pending(已认证不能回退)", AuditStatusVerified, AuditStatusPending, false},
		{"first_pass→approved(初审通过不能转为设计通过)", AuditStatusFirstPass, AuditStatusApproved, false},
		{"rejected→approved(驳回后必须先回到待审核)", AuditStatusRejected, AuditStatusApproved, false},
		{"相同状态", AuditStatusPending, AuditStatusPending, false},
		{"未知源状态", "unknown", AuditStatusApproved, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanTransitAudit(tt.from, tt.to)
			if got != tt.expect {
				t.Errorf("CanTransitAudit(%q→%q) 期望 %v, 实际 %v", tt.from, tt.to, tt.expect, got)
			}
		})
	}
}

func TestIsAuditTerminalStatus(t *testing.T) {
	// approved、final_pass、verified 是终态
	terminal := []string{AuditStatusApproved, AuditStatusFinalPass, AuditStatusVerified}
	for _, s := range terminal {
		if !IsAuditTerminalStatus(s) {
			t.Errorf("状态 %q 应为终态", s)
		}
	}

	// 其他状态不是终态
	nonTerminal := []string{AuditStatusPending, AuditStatusFirstPass, AuditStatusRejected, "unknown"}
	for _, s := range nonTerminal {
		if IsAuditTerminalStatus(s) {
			t.Errorf("状态 %q 不应为终态", s)
		}
	}
}
