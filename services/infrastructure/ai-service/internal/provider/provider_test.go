package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMockIsClearlyMarked(t *testing.T) {
	resp, err := (Mock{}).Complete(context.Background(), Request{Messages: []Message{{Role: "user", Content: "最近事业如何"}}})
	if err != nil || !strings.HasPrefix(resp.Content, "[本地开发模拟]") {
		t.Fatalf("unexpected mock response: %#v %v", resp, err)
	}
}

func TestOpenAICompatible(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer key" {
			t.Error("missing authorization")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"回答"}}],"usage":{"prompt_tokens":7,"completion_tokens":5}}`))
	}))
	defer server.Close()
	resp, err := NewOpenAICompatible(server.URL, "key", "model").Complete(context.Background(), Request{Messages: []Message{{Role: "user", Content: "问题"}}})
	if err != nil || resp.Content != "回答" || resp.TotalTokens() != 12 {
		t.Fatalf("unexpected response: %#v %v", resp, err)
	}
}

func TestOpenAICompatibleStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"答\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"案\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()
	var streamed strings.Builder
	resp, err := NewOpenAICompatible(server.URL, "key", "model").Stream(context.Background(), Request{}, func(delta string) error {
		streamed.WriteString(delta)
		return nil
	})
	if err != nil || streamed.String() != "答案" || resp.TotalTokens() != 5 || resp.FinishReason != "stop" {
		t.Fatalf("unexpected stream response: %#v %q %v", resp, streamed.String(), err)
	}
}

func TestProviderConfigValidation(t *testing.T) {
	if _, err := New(Config{Provider: "openai_compatible"}); err == nil {
		t.Fatal("missing provider config accepted")
	}
	if _, err := New(Config{Provider: "unknown"}); err == nil {
		t.Fatal("unknown provider accepted")
	}
}
