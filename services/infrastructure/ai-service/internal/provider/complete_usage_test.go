package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCompletePreservesUsageAndActualFinishReason(t *testing.T) {
	for _, tc := range []struct {
		name, choice, content, finish string
		wantError                     bool
	}{
		{"empty answer", `[{"message":{"content":"","reasoning_content":"PRIVATE REASONING"},"finish_reason":"length"}]`, "", "length", true},
		{"whitespace answer", `[{"message":{"content":" \t"},"finish_reason":"content_filter"}]`, " \t", "content_filter", true},
		{"no choices", `[]`, "", "", true},
		{"completed answer", `[{"message":{"content":"answer"},"finish_reason":"stop"}]`, "answer", "stop", false},
		{"limited answer", `[{"message":{"content":"answer"},"finish_reason":"length"}]`, "answer", "length", false},
		{"filtered answer", `[{"message":{"content":"answer"},"finish_reason":"content_filter"}]`, "answer", "content_filter", false},
		{"missing finish reason", `[{"message":{"content":"answer"}}]`, "answer", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"model":"upstream-model","choices":%s,"usage":{"prompt_tokens":37,"completion_tokens":2048,"prompt_cache_hit_tokens":11,"prompt_cache_miss_tokens":26,"completion_tokens_details":{"reasoning_tokens":2048}}}`, tc.choice)
			}))
			defer server.Close()
			out, err := NewOpenAICompatible(server.URL, "fixture", "requested-model", "").Complete(context.Background(), Request{})
			if tc.wantError {
				if !errors.Is(err, ErrEmptyContent) || strings.Contains(err.Error(), "PRIVATE") {
					t.Fatalf("invalid empty-answer error: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if out == nil || out.Content != tc.content || out.FinishReason != tc.finish || out.Model != "upstream-model" || out.PromptTokens != 37 || out.CompletionTokens != 2048 || out.PromptCacheHitTokens != 11 || out.PromptCacheMissTokens != 26 || out.ReasoningTokens != 2048 {
				t.Fatalf("lost completion metadata: %#v", out)
			}
		})
	}
}
