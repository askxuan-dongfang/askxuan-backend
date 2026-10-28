package askagent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func TestCalculatedReportConcludesOnFourthModelCall(t *testing.T) {
	for _, calculated := range []bool{true, false} {
		t.Run(fmt.Sprint("calculated=", calculated), func(t *testing.T) {
			var calls, reads atomic.Int32
			const body = `{"summary":"合成摘要","content":"## 合成报告\n已有依据足够，完成正文。"}`
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					Tools           []json.RawMessage `json:"tools"`
					ToolChoice      string            `json:"tool_choice"`
					ReasoningEffort string            `json:"reasoning_effort"`
					MaxTokens       int               `json:"max_tokens"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					return
				}
				n := int(calls.Add(1))
				if request.ReasoningEffort != "high" || request.MaxTokens != 512 || len(request.Tools) != 1 {
					t.Errorf("reasoning/output/tools changed on request %d: %+v", n, request)
				}
				if n < 4 && request.ToolChoice == "none" {
					t.Errorf("planning ended early on request %d", n)
				}
				delta, finish := map[string]any{"role": "assistant", "content": body}, "stop"
				count := 0
				if n < 4 {
					count = 1
					if calculated {
						count = []int{3, 3, 2}[n-1]
					}
				} else if n == 4 {
					if calculated && request.ToolChoice != "none" {
						// Without forced conclusion this fixture asks for a ninth tool.
						count = 1
					}
					if !calculated && request.ToolChoice == "none" {
						t.Error("purchased report tool choice changed")
					}
				} else {
					t.Errorf("model budget expanded to request %d", n)
				}
				if count > 0 {
					tools := []any{}
					for i := 0; i < count; i++ {
						tools = append(tools, map[string]any{"index": i, "id": fmt.Sprintf("read-%d-%d", n, i), "type": "function", "function": map[string]any{"name": "read_report", "arguments": "{}"}})
					}
					delta, finish = map[string]any{"role": "assistant", "tool_calls": tools}, "tool_calls"
				}
				w.Header().Set("Content-Type", "text/event-stream")
				event, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}})
				fmt.Fprintf(w, "data: %s\n\n", event)
				fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":11,\"completion_tokens\":7,\"completion_tokens_details\":{\"reasoning_tokens\":3}}}\n\ndata: [DONE]\n\n")
			}))
			defer server.Close()
			p := provider.NewOpenAICompatible(server.URL, "fixture", "deepseek-v4-pro", "")
			chat, err := provider.NewEinoModel(context.Background(), p, provider.Request{MaxTokens: 512, ThinkingEnabled: true, ReasoningEffort: "high"})
			if err != nil {
				t.Fatal(err)
			}
			out, err := Execute(context.Background(), chat, Input{Question: "合成报告", Messages: []*schema.Message{schema.UserMessage("依据现有计算完成正文")}, Report: &Report{Title: "合成依据", Content: "已完成计算", Calculated: calculated}, ContextWindow: 32768, OutputTokens: 512, Timeout: time.Minute}, nil, agent.NewGuard(20000, nil), Hooks{Call: func(_ context.Context, _, _, _ string, call func() (string, error)) (string, error) {
				reads.Add(1)
				return call()
			}})
			wantTools := 3
			if calculated {
				wantTools = 8
			}
			if err != nil || out.Text != body || calls.Load() != 4 || out.ModelCalls != 4 || out.ToolCalls != wantTools || int(reads.Load()) != wantTools || out.Usage.PromptTokens != 44 || out.Usage.CompletionTokens != 28 || out.Usage.ReasoningTokens != 12 {
				t.Fatalf("bounded conclusion/metering failed: calls=%d reads=%d out=%+v err=%v", calls.Load(), reads.Load(), out, err)
			}
		})
	}
}

func TestCalculatedReportConclusionCountsGenerateAndStreamTogether(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Stream     bool   `json:"stream"`
			ToolChoice string `json:"tool_choice"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		n := calls.Add(1)
		if (n == 4) != (request.ToolChoice == "none") {
			t.Errorf("mixed Generate/Stream request %d tool_choice=%q", n, request.ToolChoice)
		}
		if request.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"合成回答\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":2,\"completion_tokens\":1}}\n\ndata: [DONE]\n\n")
		} else {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"choices":[{"index":0,"message":{"role":"assistant","content":"合成回答"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":1}}`)
		}
	}))
	defer server.Close()
	p := provider.NewOpenAICompatible(server.URL, "fixture", "deepseek-v4-pro", "")
	chat, err := provider.NewEinoModel(context.Background(), p, provider.Request{MaxTokens: 128})
	if err != nil {
		t.Fatal(err)
	}
	// Execute configures this same flag only for current calculated reports.
	meter := &meteredModel{BaseChatModel: chat, window: 32768, output: 128, calculatedReport: true}
	input := []*schema.Message{schema.UserMessage("合成请求")}
	tools := model.WithTools([]*schema.ToolInfo{{Name: "read_report", Desc: "合成工具", ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{})}})
	for i := 0; i < 4; i++ {
		if i%2 == 0 {
			if _, err = meter.Generate(context.Background(), input, tools); err != nil {
				t.Fatal(err)
			}
		} else {
			stream, err := meter.Stream(context.Background(), input, tools)
			if err != nil {
				t.Fatal(err)
			}
			for {
				_, err = stream.Recv()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			stream.Close()
		}
	}
	u := meter.usage()
	if calls.Load() != 4 || u.PromptTokens != 8 || u.CompletionTokens != 4 {
		t.Fatalf("mixed calls or usage lost: calls=%d usage=%+v", calls.Load(), u)
	}
}
