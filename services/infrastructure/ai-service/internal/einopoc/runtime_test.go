package einopoc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/settings"
	"github.com/cloudwego/eino/components/tool"
)

var sampleSkill = model.AISkill{Code: "bazi", Status: model.SkillStatusEnabled, InputSchema: `{"fields":[{"key":"birthDate","type":"date","required":true},{"key":"birthTime","type":"time","required":true},{"key":"gender","type":"select","required":true,"options":[{"value":"male"},{"value":"female"}]}]}`, ToolConfig: `{"enabled":true,"server":"fixture","tool":"bazi"}`}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

const sampleInputs = `{"birthDate":"1990-01-02","birthTime":"12:30","gender":"male"}`

type wireRequest struct {
	Model     string            `json:"model"`
	Stream    bool              `json:"stream"`
	Tools     []any             `json:"tools"`
	Messages  []json.RawMessage `json:"messages"`
	MaxTokens int               `json:"max_tokens"`
}

func sse(w http.ResponseWriter, delta any, reason any) {
	b, _ := json.Marshal(map[string]any{"id": "probe", "object": "chat.completion.chunk", "model": "fixture", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": reason}}})
	fmt.Fprintf(w, "data: %s\n\n", b)
	w.(http.Flusher).Flush()
}
func call(w http.ResponseWriter, name, args string) {
	w.Header().Set("Content-Type", "text/event-stream")
	// A fragmented JSON argument must be reassembled by the real Eino adapter.
	half := len(args) / 2
	sse(w, map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": "c1", "type": "function", "function": map[string]any{"name": name, "arguments": args[:half]}}}}, nil)
	sse(w, map[string]any{"tool_calls": []any{map[string]any{"index": 0, "function": map[string]any{"arguments": args[half:]}}}}, "tool_calls")
	fmt.Fprint(w, "data: [DONE]\n\n")
}
func answer(w http.ResponseWriter, parts ...string) {
	w.Header().Set("Content-Type", "text/event-stream")
	for _, s := range parts {
		sse(w, map[string]any{"role": "assistant", "content": s}, nil)
	}
	sse(w, map[string]any{}, "stop")
	fmt.Fprint(w, "data: [DONE]\n\n")
}

func setup(t *testing.T, inputs string, limits Limits, reply func(http.ResponseWriter, *http.Request, int, wireRequest), failMCP bool) (*Harness, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	var models, tools atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/chat/completions":
			if r.Header.Get("X-Probe-Transport") != "pinned" {
				t.Error("existing provider HTTP transport was bypassed")
			}
			if r.Header.Get("Authorization") != "Bearer fixture-secret" {
				t.Error("lost provider authentication")
			}
			var req wireRequest
			if json.NewDecoder(r.Body).Decode(&req) != nil {
				t.Error("invalid request")
			}
			if !req.Stream || req.Model != "fixture" || len(req.Tools) != 1 || req.MaxTokens != 128 {
				t.Errorf("lost tool/model/stream/output-limit settings: %+v", req)
			}
			reply(w, r, int(models.Add(1)), req)
		case "/mcp":
			n := tools.Add(1)
			var req struct {
				Method string `json:"method"`
				Params struct {
					Name      string         `json:"name"`
					Arguments map[string]any `json:"arguments"`
				} `json:"params"`
			}
			if json.NewDecoder(r.Body).Decode(&req) != nil {
				t.Error("invalid MCP request")
			}
			if req.Method != "tools/call" || req.Params.Name != "bazi" || req.Params.Arguments["birthYear"] != float64(1990) {
				t.Errorf("unexpected MCP mapping: %+v", req)
			}
			if failMCP && n == 1 {
				http.Error(w, "UPSTREAM_SECRET", 503)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"jsonrpc":"2.0","result":{"content":[{"type":"text","text":"fixture-calculation-evidence"}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	p := provider.NewOpenAICompatible(server.URL+"/v1", "fixture-secret", "fixture", "")
	client := server.Client()
	transport := client.Transport
	client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r = r.Clone(r.Context())
		r.Header.Set("X-Probe-Transport", "pinned")
		return transport.RoundTrip(r)
	})
	p.SetHTTPClient(client)
	p.UseStandardParameters()
	chat, err := provider.NewEinoModel(context.Background(), p, provider.Request{MaxTokens: 128})
	if err != nil {
		t.Fatal(err)
	}
	guard := agent.NewGuard(2000, []string{"禁止内容"})
	var values map[string]any
	if err = json.Unmarshal([]byte(inputs), &values); err != nil {
		t.Fatal(err)
	}
	skillTool, err := NewSkillTool(sampleSkill, values, "请解释这份测试资料", agent.NewMCPClient(true, server.URL+"/mcp", 2), guard)
	if err != nil {
		t.Fatal(err)
	}
	h, err := New(context.Background(), chat, []tool.BaseTool{skillTool}, guard, limits)
	if err != nil {
		t.Fatal(err)
	}
	return h, &models, &tools
}
func defaults() Limits { return Limits{ModelCalls: 4, ToolCalls: 4, Timeout: 5 * time.Second} }

func TestMultiStepMCPStreamingAndRecovery(t *testing.T) {
	h, models, tools := setup(t, sampleInputs, defaults(), func(w http.ResponseWriter, r *http.Request, n int, req wireRequest) {
		encoded, _ := json.Marshal(req.Messages)
		all := string(encoded)
		if strings.Contains(all, "UPSTREAM_SECRET") {
			t.Error("upstream secret leaked into model context")
		}
		switch n {
		case 1:
			call(w, "calculate_bazi", "{}")
		case 2:
			if !strings.Contains(all, "calculation_unavailable") {
				t.Error("failure was not provided to the next model turn")
			}
			call(w, "calculate_bazi", "{}")
		default:
			if !strings.Contains(all, "fixture-calculation-evidence") {
				t.Error("successful evidence was not provided to model")
			}
			answer(w, "依据工具结果，", "资料已核验。")
		}
	}, true)
	var deltas []string
	result, err := h.Run(context.Background(), "解释测试资料", func(s string) error { deltas = append(deltas, s); return nil })
	if err != nil {
		t.Fatal(err)
	}
	if result.Text != "依据工具结果，资料已核验。" || strings.Join(deltas, "") != result.Text || models.Load() != 3 || tools.Load() != 2 {
		t.Fatalf("result=%+v deltas=%v models=%d tools=%d", result, deltas, models.Load(), tools.Load())
	}
}

func TestClarificationResumeWithoutInventingFacts(t *testing.T) {
	h, models, tools := setup(t, "{}", defaults(), func(w http.ResponseWriter, r *http.Request, n int, _ wireRequest) {
		if n <= 2 {
			call(w, "calculate_bazi", "{}")
		} else {
			answer(w, "补充资料后完成")
		}
	}, true)
	first, err := h.Run(context.Background(), "帮我解读", nil)
	if err != nil {
		t.Fatal(err)
	}
	if first.InterruptID == "" || first.Question == "" || tools.Load() != 0 || models.Load() != 1 {
		t.Fatalf("missing interruption: %+v", first)
	}
	if _, err = h.Resume(context.Background(), "wrong-owner-or-id", sampleInputs, nil); err == nil {
		t.Fatal("accepted wrong interruption")
	}
	second, err := h.Resume(context.Background(), first.InterruptID, "{}", nil)
	if err != nil {
		t.Fatal(err)
	}
	if second.InterruptID == "" || tools.Load() != 0 {
		t.Fatal("invalid resumed input reached MCP")
	}
	last, err := h.Resume(context.Background(), second.InterruptID, sampleInputs, nil)
	if err != nil {
		t.Fatal(err)
	}
	if last.Text != "补充资料后完成" || tools.Load() != 2 || models.Load() != 3 {
		t.Fatalf("resume lost state: %+v models=%d tools=%d", last, models.Load(), tools.Load())
	}
	if _, err = h.Resume(context.Background(), second.InterruptID, sampleInputs, nil); err == nil {
		t.Fatal("completed checkpoint replay accepted")
	}
}

func TestToolAllowlistAndModelArgumentRejection(t *testing.T) {
	for _, tc := range []struct{ name, args string }{{"execute_shell", "{}"}, {"calculate_bazi", `{"birthDate":"2000-01-01"}`}} {
		t.Run(tc.name+tc.args, func(t *testing.T) {
			h, _, tools := setup(t, sampleInputs, defaults(), func(w http.ResponseWriter, r *http.Request, n int, _ wireRequest) { call(w, tc.name, tc.args) }, false)
			if _, err := h.Run(context.Background(), "测试", nil); err == nil {
				t.Fatal("unsafe tool call accepted")
			}
			if tools.Load() != 0 {
				t.Fatal("unsafe call reached MCP")
			}
		})
	}
}

func TestExecutionBudgets(t *testing.T) {
	for _, limits := range []Limits{{ModelCalls: 2, ToolCalls: 8, Timeout: time.Second}, {ModelCalls: 8, ToolCalls: 1, Timeout: time.Second}} {
		t.Run(fmt.Sprint(limits), func(t *testing.T) {
			h, models, tools := setup(t, sampleInputs, limits, func(w http.ResponseWriter, r *http.Request, n int, _ wireRequest) { call(w, "calculate_bazi", "{}") }, false)
			if _, err := h.Run(context.Background(), "测试", nil); err == nil {
				t.Fatal("unbounded run succeeded")
			}
			if int(models.Load()) > limits.ModelCalls || int(tools.Load()) > limits.ToolCalls {
				t.Fatal("budget exceeded")
			}
		})
	}
}

func TestTimeoutAndGuardAcrossStreamChunks(t *testing.T) {
	t.Run("timeout", func(t *testing.T) {
		limits := defaults()
		limits.Timeout = 50 * time.Millisecond
		h, _, tools := setup(t, sampleInputs, limits, func(w http.ResponseWriter, r *http.Request, n int, _ wireRequest) {
			select {
			case <-r.Context().Done():
			case <-time.After(time.Second):
			}
		}, false)
		if _, err := h.Run(context.Background(), "测试", nil); err == nil {
			t.Fatal("timeout ignored")
		}
		if tools.Load() != 0 {
			t.Fatal("tool called after timeout")
		}
	})
	t.Run("guard", func(t *testing.T) {
		h, _, _ := setup(t, sampleInputs, defaults(), func(w http.ResponseWriter, r *http.Request, n int, _ wireRequest) { answer(w, "禁止", "内容") }, false)
		var visible string
		if _, err := h.Run(context.Background(), "测试", func(s string) error { visible += s; return nil }); err == nil {
			t.Fatal("unsafe output accepted")
		}
		if strings.Contains(visible, "禁止内容") {
			t.Fatal("unsafe delta escaped guard")
		}
	})
}

func TestSnapshotModelAllowlist(t *testing.T) {
	p := provider.NewOpenAICompatible("https://example.invalid/v1", "fixture", "fixture", "")
	p.SetHTTPClient(&http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected model execution: %s", r.URL.Path)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"fixture"},{"id":"not-allowed"}]}`))}, nil
	})})
	s := &settings.Snapshot{Provider: p, Models: provider.NewCatalogWithAllowed(p, []string{"fixture"})}
	if _, err := FromSnapshot(context.Background(), s, "not-allowed", nil, defaults()); !errors.Is(err, provider.ErrModelUnavailable) {
		t.Fatalf("model allowlist bypassed: %v", err)
	}
}

func TestCheckpointRestoresAcrossHarnessInstances(t *testing.T) {
	reply := func(w http.ResponseWriter, r *http.Request, n int, req wireRequest) {
		b, _ := json.Marshal(req.Messages)
		if strings.Contains(string(b), "fixture-calculation-evidence") {
			answer(w, "恢复后的有据参考")
		} else {
			call(w, "calculate_bazi", "{}")
		}
	}
	first, _, firstTools := setup(t, `{}`, defaults(), reply, false)
	out, err := first.Run(context.Background(), "合成资料测试", nil)
	if err != nil || out.InterruptID == "" {
		t.Fatalf("pause: %+v %v", out, err)
	}
	cp, err := first.Checkpoint()
	if err != nil {
		t.Fatal(err)
	}
	second, _, secondTools := setup(t, `{}`, defaults(), reply, false)
	if err = second.Restore(cp); err != nil {
		t.Fatal(err)
	}
	out, err = second.Resume(context.Background(), cp.InterruptID, sampleInputs, nil)
	if err != nil || out.InterruptID != "" || out.Text != "恢复后的有据参考" || firstTools.Load() != 0 || secondTools.Load() != 1 {
		t.Fatalf("restore: %+v %v", out, err)
	}
	mc, tc := second.Counts()
	if mc < cp.Models || tc < cp.Tools {
		t.Fatal("restored budget reset")
	}
	if err = second.Restore(cp); err == nil {
		t.Fatal("overwrote started runtime")
	}
}
