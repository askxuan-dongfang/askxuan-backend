package askagent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/cloudwego/eino/schema"
)

func TestThinkingToolsReplayReasoningWithoutPublishingIt(t *testing.T) {
	const privateThought = "PRIVATE_REASONING_TOOL_ROUND"
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			t.Error("invalid fixture request")
			return
		}
		thinking, _ := req["thinking"].(map[string]any)
		n := calls.Add(1)
		if n == 3 {
			if req["model"] != "chat-default" || thinking["type"] != "disabled" || req["reasoning_effort"] != nil {
				t.Error("report request changed ordinary chat policy")
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"choices":[{"message":{"content":"普通聊天回答"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":2}}`)
			return
		}
		if req["model"] != "report-model" || thinking["type"] != "enabled" || req["reasoning_effort"] != "high" || req["stream"] != true {
			t.Error("thinking/tool policy changed between report model calls")
		}
		var delta map[string]any
		reason, promptTokens, completionTokens, reasoningTokens := "tool_calls", 11, 7, 5
		if n == 1 {
			delta = map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": "report-read", "type": "function", "function": map[string]any{"name": "read_report", "arguments": "{}"}}}}
		} else {
			messages, _ := req["messages"].([]any)
			replayed, toolResult := false, false
			for _, raw := range messages {
				m, _ := raw.(map[string]any)
				if m["role"] == "assistant" && m["reasoning_content"] == privateThought {
					replayed = true
				}
				if m["role"] == "tool" && m["tool_call_id"] == "report-read" {
					toolResult = true
				}
			}
			if !replayed || !toolResult {
				t.Error("Eino tool round lost its private reasoning or tool result")
			}
			delta = map[string]any{"role": "assistant", "content": "只含可见结论"}
			reason, promptTokens, completionTokens, reasoningTokens = "stop", 17, 13, 9
		}
		w.Header().Set("Content-Type", "text/event-stream")
		thought := privateThought
		if n == 2 {
			thought = "PRIVATE_REASONING_FINAL_ROUND"
		}
		thinkingEvent, _ := json.Marshal(map[string]any{"id": "fixture", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "reasoning_content": thought}}}})
		event, _ := json.Marshal(map[string]any{"id": "fixture", "model": "report-model", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": reason}}, "usage": map[string]any{"prompt_tokens": promptTokens, "completion_tokens": completionTokens, "completion_tokens_details": map[string]any{"reasoning_tokens": reasoningTokens}}})
		fmt.Fprintf(w, "data: %s\n\ndata: %s\n\ndata: [DONE]\n\n", thinkingEvent, event)
	}))
	defer server.Close()
	p := provider.NewOpenAICompatible(server.URL, "fixture", "chat-default", "")
	chat, err := provider.NewEinoModel(context.Background(), p, provider.Request{Model: "report-model", ThinkingEnabled: true, ReasoningEffort: "high", MaxTokens: 8192})
	if err != nil {
		t.Fatal(err)
	}
	var published []string
	out, err := Execute(context.Background(), chat, Input{Question: "合成任务", Messages: []*schema.Message{schema.UserMessage("读取本轮依据后作答")}, Report: &Report{Title: "本轮依据", Content: `{"事实":"合成数据"}`, Calculated: true}, ContextWindow: 32768, OutputTokens: 8192, Timeout: time.Minute}, nil, agent.NewGuard(20000, nil), Hooks{Text: func(s string) error { published = append(published, s); return nil }})
	if err != nil || out.Text != "只含可见结论" || out.ModelCalls != 2 || out.ToolCalls != 1 || out.Usage.PromptTokens != 28 || out.Usage.CompletionTokens != 20 || out.Usage.ReasoningTokens != 14 {
		t.Fatalf("thinking tool workflow failed: output=%q calls=%d tools=%d usage=%+v err=%v", out.Text, out.ModelCalls, out.ToolCalls, out.Usage, err)
	}
	public, _ := json.Marshal(struct {
		Text   string
		Events []string
	}{out.Text, published})
	if strings.Contains(string(public), "PRIVATE_REASONING") {
		t.Fatal("reasoning entered visible answer/events")
	}
	if _, err := p.Complete(context.Background(), provider.Request{Messages: []provider.Message{{Role: "user", Content: "普通聊天"}}, MaxTokens: 64}); err != nil || calls.Load() != 3 || p.Model() != "chat-default" {
		t.Fatal("ordinary chat defaults changed after reasoning report", err)
	}
}
