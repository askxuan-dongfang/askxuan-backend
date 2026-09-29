package askagent

import (
	"encoding/json"
	"github.com/askxuan/ai-service/internal/agent"
	business "github.com/askxuan/ai-service/internal/model"
	"github.com/cloudwego/eino/schema"
	"os"
	"testing"
)

func TestAllReviewedToolsRunInHarnessAndAskForMissingFacts(t *testing.T) {
	raw, e := os.ReadFile("../agent/testdata/tool_inputs.json")
	if e != nil {
		t.Fatal(e)
	}
	var fixtures map[string]map[string]any
	_ = json.Unmarshal(raw, &fixtures)
	skills := []*business.AISkill{}
	for _, d := range agent.ReviewedTools() {
		fields, _ := json.Marshal(d.InputSchema)
		tc, _ := json.Marshal(map[string]any{"enabled": true, "server": "fixture", "tool": d.Code})
		skills = append(skills, &business.AISkill{Code: d.Code, Name: d.Name, Description: d.Description, Status: "enabled", InputSchema: string(fields), ToolConfig: string(tc)})
	}
	partial, e := PartialSchema(skills)
	if e != nil {
		t.Fatal(e)
	}
	for _, s := range skills {
		t.Run(s.Code, func(t *testing.T) {
			facts := fixtures[s.Code]
			if _, e := agent.NewGuard(20000, nil).Validate(partial, "合成测试", facts); e != nil {
				t.Fatalf("partial facts rejected: %v", e)
			}
			in := Input{Question: "合成测试", Skills: skills, Facts: facts, Messages: []*schema.Message{schema.UserMessage("合成测试")}}
			m := &mcpStub{}
			out, e := runFixture(t, in, m, []map[string]any{toolCall("calculate_"+s.Code, "{}"), {"role": "assistant", "content": "工具已返回测试依据。"}})
			if e != nil || m.calls != 1 || out.Clarification != nil {
				t.Fatalf("cannot use tool: %+v %v calls=%d", out, e, m.calls)
			}
			in.Facts = map[string]any{}
			m = &mcpStub{}
			out, e = runFixture(t, in, m, []map[string]any{toolCall("calculate_"+s.Code, "{}")})
			if e != nil || m.calls != 0 || out.Clarification == nil {
				t.Fatalf("missing data not clarified: %+v %v", out, e)
			}
		})
	}
}
func TestInactiveConditionalFactsDoNotLeakBetweenTools(t *testing.T) {
	raw := `{"fields":[{"key":"method","type":"text"},{"key":"numbers","type":"text","visibleWhen":{"key":"method","value":"number"}}]}`
	got := FilterFacts(raw, map[string]any{"method": "time", "numbers": "1 2", "unrelated": "secret"})
	if len(got) != 1 || got["method"] != "time" {
		t.Fatalf("inactive data leaked: %+v", got)
	}
}
