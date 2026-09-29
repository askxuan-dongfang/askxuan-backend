package askagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/askxuan/ai-service/internal/agent"
	business "github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/cloudwego/eino/schema"
)

var bazi = &business.AISkill{Code: "bazi", Name: "八字", Status: "enabled", InputSchema: `{"fields":[{"key":"birthDate","label":"出生日期","type":"date","required":true},{"key":"birthTime","label":"出生时间","type":"time","required":true},{"key":"gender","label":"性别","type":"select","required":true,"options":[{"value":"male","label":"男"},{"value":"female","label":"女"}]}]}`, ToolConfig: `{"enabled":true,"server":"fixture","tool":"bazi"}`}

type mcpStub struct {
	calls  int
	fail   int
	result string
}

func (m *mcpStub) Call(_ context.Context, _, args string) (string, error) {
	m.calls++
	if strings.Contains(args, "2099") {
		panic("fabricated birth facts")
	}
	if m.calls <= m.fail {
		return "", errors.New("private upstream error")
	}
	if m.result != "" {
		return m.result, nil
	}
	return `{"yearPillar":"fixture-evidence"}`, nil
}
func runFixture(t *testing.T, input Input, mcp *mcpStub, replies []map[string]any) (Outcome, error) {
	t.Helper()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Messages []json.RawMessage `json:"messages"`
			Tools    []json.RawMessage `json:"tools"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			t.Error("invalid request")
		}
		raw, _ := json.Marshal(req.Messages)
		if strings.Contains(string(raw), "private upstream error") {
			t.Error("upstream secret leaked")
		}
		i := int(calls.Add(1)) - 1
		if i >= len(replies) {
			t.Error("unexpected model call")
			http.Error(w, "fixture exhausted", 500)
			return
		}
		delta := replies[i]
		reason := "stop"
		if _, ok := delta["tool_calls"]; ok {
			reason = "tool_calls"
		}
		w.Header().Set("Content-Type", "text/event-stream")
		event, _ := json.Marshal(map[string]any{"id": "fixture", "object": "chat.completion.chunk", "model": "fixture", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": reason}}})
		fmt.Fprintf(w, "data: %s\n\n", event)
		fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5}}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	p := provider.NewOpenAICompatible(server.URL, "fixture", "fixture", "")
	p.SetHTTPClient(server.Client())
	p.UseStandardParameters()
	chat, e := provider.NewEinoModel(context.Background(), p, provider.Request{MaxTokens: 128})
	if e != nil {
		t.Fatal(e)
	}
	return Execute(context.Background(), chat, input, mcp, agent.NewGuard(4000, nil), Hooks{})
}
func toolCall(name, args string) map[string]any {
	return map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": "call-" + name, "type": "function", "function": map[string]any{"name": name, "arguments": args}}}}
}
func fixtureInput() Input {
	return Input{Question: "请根据报告补充分析", Messages: []*schema.Message{schema.UserMessage("请根据报告补充分析")}, Skills: []*business.AISkill{bazi}, Facts: map[string]any{"birthDate": "1990-01-02", "birthTime": "12:30", "gender": "male"}, Report: &Report{Title: "本地报告", Content: "已解锁的虚构报告"}}
}
func TestLiveLoopReadsReportCalculatesThenAnswers(t *testing.T) {
	m := &mcpStub{}
	o, e := runFixture(t, fixtureInput(), m, []map[string]any{toolCall("read_report", "{}"), toolCall("calculate_bazi", "{}"), {"role": "assistant", "content": "根据报告和八字工具的 fixture-evidence 整理回答。"}})
	if e != nil || o.ModelCalls != 3 || o.ToolCalls != 2 || m.calls != 1 || !strings.Contains(o.Text, "fixture-evidence") || o.Usage.PromptTokens != 30 || o.Usage.CompletionTokens != 15 {
		t.Fatalf("not a real tool loop: %+v %v calls=%d", o, e, m.calls)
	}
}
func TestMissingFactsInterruptAndNextTurnContinues(t *testing.T) {
	in := fixtureInput()
	delete(in.Facts, "birthTime")
	m := &mcpStub{}
	o, e := runFixture(t, in, m, []map[string]any{toolCall("calculate_bazi", "{}")})
	if e != nil || o.Clarification == nil || !strings.Contains(o.Text, "出生时间") || m.calls != 0 {
		t.Fatalf("must ask instead of guess: %+v %v", o, e)
	}
	// Simulate reconstructing the next turn from stored facts after a process restart.
	in.Facts["birthTime"] = "08:30"
	in.Messages = append(in.Messages, schema.AssistantMessage(o.Text, nil), schema.UserMessage("已确认补充资料"))
	o, e = runFixture(t, in, m, []map[string]any{toolCall("calculate_bazi", "{}"), {"role": "assistant", "content": "已根据补充的资料完成计算。"}})
	if e != nil || o.Clarification != nil || m.calls != 1 {
		t.Fatalf("resume failed: %+v %v", o, e)
	}
}
func TestToolFailureCanRecoverWithoutLeakingDetails(t *testing.T) {
	m := &mcpStub{fail: 1}
	o, e := runFixture(t, fixtureInput(), m, []map[string]any{toolCall("calculate_bazi", "{}"), toolCall("calculate_bazi", "{}"), {"role": "assistant", "content": "重试后已取得依据。"}})
	if e != nil || m.calls != 2 || o.ModelCalls != 3 {
		t.Fatalf("tool recovery failed: %+v %v", o, e)
	}
}
func TestModelCannotInventFactsOrCallUnavailableTool(t *testing.T) {
	for _, call := range []map[string]any{toolCall("calculate_bazi", `{"birthDate":"2099-01-01"}`), toolCall("send_email", `{}`)} {
		m := &mcpStub{}
		_, e := runFixture(t, fixtureInput(), m, []map[string]any{call})
		if e == nil || m.calls != 0 {
			t.Fatal("unauthorized tool arguments accepted")
		}
	}
}
func TestSimpleQuestionDoesNotRequireTools(t *testing.T) {
	in := fixtureInput()
	in.Report = nil
	m := &mcpStub{}
	o, e := runFixture(t, in, m, []map[string]any{{"role": "assistant", "content": "可以先记录自己的想法。"}})
	if e != nil || o.ToolCalls != 0 || m.calls != 0 {
		t.Fatalf("unnecessary tool call: %+v %v", o, e)
	}
}
func TestRepeatedSameCalculationUsesRunCache(t *testing.T) {
	m := &mcpStub{}
	o, e := runFixture(t, fixtureInput(), m, []map[string]any{toolCall("calculate_bazi", "{}"), toolCall("calculate_bazi", "{}"), {"role": "assistant", "content": "引用同一次计算。"}})
	if e != nil || m.calls != 1 || o.ToolCalls != 2 {
		t.Fatalf("duplicate calculation: %+v %v", o, e)
	}
}
func TestPartialInputSchemaStillRejectsUnknownAndInvalidFacts(t *testing.T) {
	raw, e := PartialSchema([]*business.AISkill{bazi})
	if e != nil {
		t.Fatal(e)
	}
	g := agent.NewGuard(4000, nil)
	if _, e = g.Validate(raw, "问题", map[string]any{"birthDate": "1990-01-02"}); e != nil {
		t.Fatal(e)
	}
	for _, facts := range []map[string]any{{"birthDate": "bad"}, {"endpoint": "attacker"}, {"gender": "invalid"}} {
		if _, e = g.Validate(raw, "问题", facts); e == nil {
			t.Fatal("invalid facts accepted")
		}
	}
}

func TestReasoningBudgetRetriesOnceAndMetersBothAttempts(t *testing.T) {
	for _, partial := range []bool{false, true} {
		t.Run(fmt.Sprint(partial), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req map[string]any
				_ = json.NewDecoder(r.Body).Decode(&req)
				n := calls.Add(1)
				if n > 2 {
					t.Error("unbounded model retry")
				}
				if n == 2 {
					thinking, _ := req["thinking"].(map[string]any)
					if thinking["type"] != "disabled" {
						t.Errorf("fallback did not disable thinking: %v", thinking)
					}
				}
				if req["max_tokens"] != float64(128) {
					t.Errorf("changed token budget: %v", req["max_tokens"])
				}
				content, reason := "", "length"
				if partial {
					content = "不完整回答"
				}
				if n == 2 {
					content, reason = "已恢复并完成回答。", "stop"
				}
				w.Header().Set("Content-Type", "text/event-stream")
				event, _ := json.Marshal(map[string]any{"id": "fixture", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "content": content}, "finish_reason": reason}}})
				fmt.Fprintf(w, "data: %s\n\n", event)
				fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":128,\"completion_tokens_details\":{\"reasoning_tokens\":100}}}\n\ndata: [DONE]\n\n")
			}))
			defer server.Close()
			p := provider.NewOpenAICompatible(server.URL, "fixture", "fixture", "")
			p.SetHTTPClient(server.Client())
			chat, e := provider.NewEinoModel(context.Background(), p, provider.Request{MaxTokens: 128, ThinkingEnabled: true})
			if e != nil {
				t.Fatal(e)
			}
			input := fixtureInput()
			input.ReasoningFallback = provider.ReasoningFallbackOptions(p, true)
			published := ""
			out, e := Execute(context.Background(), chat, input, &mcpStub{}, agent.NewGuard(4000, nil), Hooks{Text: func(s string) error { published += s; return nil }})
			if partial {
				if e == nil || calls.Load() != 1 || published != "" {
					t.Fatalf("partial output accepted/retried: %+v %v %q", out, e, published)
				}
			} else if e != nil || calls.Load() != 2 || out.ModelCalls != 2 || out.Usage.CompletionTokens != 256 || out.Usage.ReasoningTokens != 200 || published != "已恢复并完成回答。" {
				t.Fatalf("recovery or metering failed: %+v %v calls=%d published=%q", out, e, calls.Load(), published)
			}
		})
	}
}

func TestModelLoopStopsAtBudget(t *testing.T) {
	replies := []map[string]any{}
	for i := 0; i < 6; i++ {
		replies = append(replies, toolCall("read_report", "{}"))
	}
	out, e := runFixture(t, fixtureInput(), &mcpStub{}, replies)
	if e == nil || out.ModelCalls > 4 || out.ToolCalls > 4 {
		t.Fatalf("unbounded loop: %+v %v", out, e)
	}
}

func TestConditionalToolFieldsArePreservedForClarification(t *testing.T) {
	in := fixtureInput()
	s := *bazi
	s.Code = "liuyao"
	s.ToolConfig = `{"enabled":true,"server":"taibu","tool":"liuyao"}`
	s.InputSchema = `{"fields":[{"key":"method","type":"select","required":true,"options":[{"value":"number","label":"数字"},{"value":"auto","label":"自动"}]},{"key":"numbers","type":"text"}]}`
	in.Skills = []*business.AISkill{&s}
	in.Facts = map[string]any{"method": "number"}
	m := &mcpStub{}
	out, e := runFixture(t, in, m, []map[string]any{toolCall("calculate_liuyao", "{}")})
	if e != nil || out.Clarification == nil || m.calls != 0 {
		t.Fatalf("missing conditional data bypassed: %+v %v", out, e)
	}
	field := out.Clarification.Fields[1]
	if field.RequiredWhen == nil || field.VisibleWhen == nil || field.Validation != "divination-numbers" {
		t.Fatalf("lost form contract: %+v", field)
	}
}

func TestLargeToolResultReachesModelWithinConfiguredContext(t *testing.T) {
	in := fixtureInput()
	in.ContextWindow = 1048576
	m := &mcpStub{result: strings.Repeat("大运流年依据。", 6000)}
	out, err := runFixture(t, in, m, []map[string]any{toolCall("calculate_bazi", "{}"), {"role": "assistant", "content": "已根据完整工具结果回答。"}})
	if err != nil || out.ModelCalls != 2 || out.ToolCalls != 1 || out.Text == "" {
		t.Fatalf("large result lost: %+v %v", out, err)
	}
}
