package agentops

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/askxuan/ai-service/internal/askagent"
	"github.com/askxuan/ai-service/internal/config"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/settings"
)

type debugFixtureMCP struct{ calls int }

func (m *debugFixtureMCP) Call(context.Context, string, string) (string, error) {
	m.calls++
	return `{"yearPillar":"fixture"}`, nil
}

func TestHarnessDebugSharesRuntimeAndClarificationGate(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(fmt.Sprint(missing), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/models") {
					fmt.Fprint(w, `{"data":[{"id":"fixture"}]}`)
					return
				}
				var req struct {
					MaxTokens int               `json:"max_tokens"`
					Tools     []json.RawMessage `json:"tools"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Error(err)
				}
				if req.MaxTokens != 1024 || len(req.Tools) == 0 {
					t.Errorf("draft budget/tools not applied: %+v", req)
				}
				calls++
				delta := map[string]any{"role": "assistant", "content": "根据已核验资料作文化参考。"}
				reason := "stop"
				if calls == 1 {
					delta = map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": "c1", "type": "function", "function": map[string]string{"name": "calculate_bazi", "arguments": "{}"}}}}
					reason = "tool_calls"
				}
				event, _ := json.Marshal(map[string]any{"id": "fixture", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": reason}}})
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprintf(w, "data: %s\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5}}\n\ndata: [DONE]\n\n", event)
			}))
			defer server.Close()
			p := provider.NewOpenAICompatible(server.URL, "fixture", "fixture", "")
			p.SetHTTPClient(server.Client())
			p.UseStandardParameters()
			skill := model.AISkill{Code: "bazi", Name: "八字", Status: "enabled", InputSchema: `{"fields":[{"key":"birthDate","label":"出生日期","type":"date","required":true},{"key":"birthTime","label":"出生时间","type":"time","required":true},{"key":"gender","label":"性别","type":"select","required":true,"options":[{"value":"male","label":"男"},{"value":"female","label":"女"}]}]}`, ToolConfig: `{"enabled":true,"server":"fixture","tool":"bazi"}`}
			frozen := Frozen{Config: Config{MaxOutputTokens: 1024}, Skills: []model.AISkill{skill}}
			inputs := map[string]any{}
			if !missing {
				inputs["birthDate"] = "1990-01-02"
				inputs["birthTime"] = "12:30"
				inputs["gender"] = "male"
			}
			task := &debugTask{frozen: frozen, question: "请使用八字计算工具分析", inputs: inputs, snapshot: &settings.Snapshot{Config: config.AIConf{MaxInputChars: 4000, MaxOutputTokens: 1024, ContextWindow: 32768}, Provider: p, Models: provider.NewCatalog(p)}}
			mcp := &debugFixtureMCP{}
			manager := &Manager{MCP: mcp}
			d := DebugRun{Model: "fixture", SkillCode: "bazi"}
			traces := 0
			stages := 0
			err := manager.liveExecution(context.Background(), task, &d, askagent.Hooks{Stage: func(string) error { stages++; return nil }, Call: func(ctx context.Context, server, name, args string, fn func() (string, error)) (string, error) {
				traces++
				if server != "fixture" || name != "bazi" {
					t.Error("incorrect trace identity")
				}
				return fn()
			}}, true)
			if err != nil || stages == 0 || d.ModelAttempts == 0 || d.PromptTokens == nil || *d.PromptTokens < 10 {
				t.Fatalf("missing runtime result/usage: %+v %v", d, err)
			}
			if missing {
				if d.Clarification == "" || !strings.Contains(d.Result, "重新试运行") || d.InterruptID != "" || mcp.calls != 0 {
					t.Fatalf("missing input was not safely surfaced: %+v", d)
				}
				calls = 0
				if err = manager.liveEvaluation(context.Background(), task, &DebugRun{Model: "fixture", SkillCode: "bazi"}); err == nil {
					t.Fatal("evaluation accepted unresolved clarification")
				}
			} else if d.Result == "" || traces != 1 || mcp.calls != 1 || d.ModelAttempts != 2 {
				t.Fatalf("not a real model/tool loop: %+v traces=%d calls=%d", d, traces, mcp.calls)
			}
		})
	}
}

func TestDebugMCPRejectsInvalidConfig(t *testing.T) {
	c := &debugMCP{base: &debugFixtureMCP{}, trace: func(context.Context, string, string, string, func() (string, error)) (string, error) {
		t.Fatal("invalid config traced")
		return "", nil
	}}
	if _, err := c.Call(context.Background(), "invalid", "{}"); err == nil {
		t.Fatal("invalid config accepted")
	}
}
