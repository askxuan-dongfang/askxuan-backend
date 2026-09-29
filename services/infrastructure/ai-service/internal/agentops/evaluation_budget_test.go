package agentops

import (
	"fmt"
	"testing"
	"time"
)

func TestEvaluationCoverageBeyondTwentySkills(t *testing.T) {
	cases := []EvaluationCase{}
	skills := []SkillPolicy{}
	for i := 0; i < 23; i++ {
		code := fmt.Sprintf("skill%d", i)
		skills = append(skills, SkillPolicy{Code: code, Enabled: true})
		cases = append(cases, EvaluationCase{ID: code, Name: code, SkillCode: code, Question: "合成测试", Inputs: map[string]any{}, MinChars: 20})
	}
	if err := validateCases(cases, skills, true); err != nil {
		t.Fatal(err)
	}
	if evaluationTimeout(23) != 23*time.Minute || evaluationTimeout(100) != 30*time.Minute || evaluationTimeout(1) != 3*time.Minute {
		t.Fatal("deadline must scale and stay bounded")
	}
}
