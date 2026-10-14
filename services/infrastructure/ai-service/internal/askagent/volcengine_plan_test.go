package askagent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/askxuan/ai-service/internal/agent"
	business "github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/cloudwego/eino/schema"
)

type planTestTransport struct {
	target *url.URL
	base   http.RoundTripper
}

func (p planTestTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.URL.Scheme, r.URL.Host = p.target.Scheme, p.target.Host
	return p.base.RoundTrip(r)
}

// A connection probe has no tool catalog. Exercise the production catalog ->
// limits -> Eino -> Harness path with every reviewed tool and the saved 32K
// output setting, including a second model call after a tool result.
func TestAgentPlanCatalogBudgetWithFullHarness(t *testing.T) {
	skills := []*business.AISkill{}
	for _, d := range agent.ReviewedTools() {
		fields, _ := json.Marshal(d.InputSchema)
		config, _ := json.Marshal(map[string]any{"enabled": true, "server": "fixture", "tool": d.Code})
		skills = append(skills, &business.AISkill{Code: d.Code, Name: d.Name, Description: d.Description, Status: "enabled", InputSchema: string(fields), ToolConfig: string(config)})
	}
	for _, withTool := range []bool{false, true} {
		t.Run(fmt.Sprintf("tool=%t", withTool), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body struct {
					Model     string            `json:"model"`
					MaxTokens int               `json:"max_tokens"`
					Thinking  json.RawMessage   `json:"thinking"`
					Tools     []json.RawMessage `json:"tools"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if r.URL.Path != "/api/plan/v3/chat/completions" || body.Model != "ark-code-latest" || body.MaxTokens != 32000 || body.Thinking != nil || len(body.Tools) < len(skills) {
					t.Errorf("unexpected Plan Harness request: path=%s model=%s output=%d tools=%d thinking=%s", r.URL.Path, body.Model, body.MaxTokens, len(body.Tools), body.Thinking)
				}
				delta, reason := map[string]any{"role": "assistant", "content": "你好，已完成验证。"}, "stop"
				if calls.Add(1) == 1 && withTool {
					delta, reason = toolCall("read_report", "{}"), "tool_calls"
				}
				w.Header().Set("Content-Type", "text/event-stream")
				chunk, _ := json.Marshal(map[string]any{"id": "fixture", "model": "glm-5.3", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": reason}}})
				fmt.Fprintf(w, "data: %s\n\ndata: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5}}\n\ndata: [DONE]\n\n", chunk)
			}))
			defer server.Close()
			p, err := provider.New(provider.Config{Provider: "volcengine", BaseURL: provider.VolcenginePlanBaseURL, APIKey: "fixture", Model: "ark-code-latest"})
			if err != nil {
				t.Fatal(err)
			}
			local, _ := url.Parse(server.URL)
			p.(*provider.OpenAICompatible).SetHTTPClient(&http.Client{Transport: planTestTransport{target: local, base: server.Client().Transport}})
			window, output, err := provider.NewCatalog(p).Limits(context.Background(), p.Model(), 1048576, 32768)
			if err != nil {
				t.Fatal(err)
			}
			chat, err := provider.NewEinoModel(context.Background(), p, provider.Request{Model: p.Model(), MaxTokens: output})
			if err != nil {
				t.Fatal(err)
			}
			in := Input{ContextWindow: window, OutputTokens: output, Question: "你好", Messages: []*schema.Message{schema.UserMessage("你好")}, Skills: skills, Facts: map[string]any{}, Report: &Report{Title: "合成报告", Content: "仅用于测试"}, KnowledgeEnabled: true, MemoryEnabled: true, References: func(context.Context, string, string) (string, error) { return "[]", nil }}
			out, err := Execute(context.Background(), chat, in, &mcpStub{}, agent.NewGuard(4000, nil), Hooks{})
			wantCalls, wantTools := 1, 0
			if withTool {
				wantCalls, wantTools = 2, 1
			}
			if err != nil || out.Text == "" || out.ModelCalls != wantCalls || out.ToolCalls != wantTools || int(calls.Load()) != wantCalls {
				t.Fatalf("Plan conversation failed: result=%+v err=%v upstreamCalls=%d", out, err, calls.Load())
			}
			if out.Context.EstimatedInputTokens+out.Context.OutputBudget+4096 > out.Context.Window {
				t.Fatalf("context protection bypassed: %+v", out.Context)
			}
		})
	}
}
