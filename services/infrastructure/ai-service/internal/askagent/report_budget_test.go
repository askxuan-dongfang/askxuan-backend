package askagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/einopoc"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/cloudwego/eino/schema"
)

func TestOnlyCurrentCalculatedReportsReceiveEightToolCalls(t *testing.T) {
	for _, tc := range []struct {
		calculated          bool
		requested, executed int
		blocked             bool
	}{
		{false, 5, 4, true}, {true, 5, 5, false}, {true, 9, 8, true},
	} {
		t.Run(fmt.Sprintf("calculated=%t/requested=%d", tc.calculated, tc.requested), func(t *testing.T) {
			var models, reads atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				delta := map[string]any{"role": "assistant", "content": "合成结论"}
				finish := "stop"
				if models.Add(1) == 1 {
					calls := []any{}
					for i := 0; i < tc.requested; i++ {
						calls = append(calls, map[string]any{"index": i, "id": fmt.Sprint("read-", i), "type": "function", "function": map[string]any{"name": "read_report", "arguments": "{}"}})
					}
					delta = map[string]any{"role": "assistant", "tool_calls": calls}
					finish = "tool_calls"
				}
				w.Header().Set("Content-Type", "text/event-stream")
				event, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}})
				fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", event)
			}))
			defer server.Close()
			p := provider.NewOpenAICompatible(server.URL, "fixture", "fixture-model", "")
			chat, err := provider.NewEinoModel(context.Background(), p, provider.Request{MaxTokens: 128})
			if err != nil {
				t.Fatal(err)
			}
			out, err := Execute(context.Background(), chat, Input{Question: "合成任务", Messages: []*schema.Message{schema.UserMessage("核对现有依据")}, Report: &Report{Title: "合成依据", Content: "已确认资料", Calculated: tc.calculated}, ContextWindow: 32768, OutputTokens: 128, Timeout: time.Minute}, nil, agent.NewGuard(20000, nil), Hooks{Call: func(_ context.Context, _, _, _ string, call func() (string, error)) (string, error) {
				reads.Add(1)
				return call()
			}})
			if reads.Load() != int32(tc.executed) || errors.Is(err, einopoc.ErrBudget) != tc.blocked {
				t.Fatalf("tool budget changed: reads=%d err=%v", reads.Load(), err)
			}
			if !tc.blocked && (err != nil || out.Text != "合成结论" || out.ModelCalls != 2 || out.ToolCalls != 5) {
				t.Fatalf("bounded calculated report failed: %+v %v", out, err)
			}
		})
	}
}
