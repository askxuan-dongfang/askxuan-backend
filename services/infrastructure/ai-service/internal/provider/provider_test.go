package provider

import (
	"context"
	"encoding/json"
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
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["model"] != "vision" {
			t.Errorf("model=%v, want vision", payload["model"])
		}
		thinking, _ := payload["thinking"].(map[string]interface{})
		if thinking["type"] != "enabled" || payload["reasoning_effort"] != "low" {
			t.Errorf("thinking payload=%v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"回答"}}],"usage":{"prompt_tokens":7,"completion_tokens":5}}`))
	}))
	defer server.Close()
	resp, err := NewOpenAICompatible(server.URL, "key", "model", "vision").Complete(context.Background(), Request{
		ThinkingEnabled: true,
		ReasoningEffort: "low",
		Messages:        []Message{{Role: "user", Content: "问题", ImageDataURLs: []string{"data:image/png;base64,eA=="}}},
	})
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
	resp, err := NewOpenAICompatible(server.URL, "key", "model", "").Stream(context.Background(), Request{}, func(delta StreamDelta) error {
		streamed.WriteString(delta.Content)
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

func TestStreamPreservesUsageAfterMetadataOnlyChunks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"answer\"}}],\"usage\":{\"prompt_tokens\":11,\"completion_tokens\":9,\"completion_tokens_details\":{\"reasoning_tokens\":4}}}\n\ndata: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n"))
	}))
	defer server.Close()
	resp, err := NewOpenAICompatible(server.URL, "key", "model", "").Stream(context.Background(), Request{}, func(StreamDelta) error { return nil })
	if err != nil || resp.TotalTokens() != 20 || resp.ReasoningTokens != 4 {
		t.Fatalf("lost usage: %#v %v", resp, err)
	}
}

func TestReasoningOnlyStreamReportsBudgetAndUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"private reasoning\"}}]}\n\ndata: {\"choices\":[{\"delta\":{},\"finish_reason\":\"length\"}],\"usage\":{\"prompt_tokens\":37,\"completion_tokens\":2048,\"completion_tokens_details\":{\"reasoning_tokens\":2048}}}\n\ndata: [DONE]\n\n"))
	}))
	defer server.Close()
	var exposed strings.Builder
	resp, err := NewOpenAICompatible(server.URL, "key", "model", "").Stream(context.Background(), Request{ThinkingEnabled: true}, func(d StreamDelta) error { exposed.WriteString(d.Content); return nil })
	if err == nil || !strings.Contains(err.Error(), "finish_reason=length") || resp == nil || resp.CompletionTokens != 2048 || resp.ReasoningTokens != 2048 || exposed.Len() != 0 {
		t.Fatalf("invalid reasoning-only handling: %#v %v", resp, err)
	}
	if strings.Contains(err.Error(), "private reasoning") {
		t.Fatal("reasoning leaked")
	}
}

func TestStreamErrorDoesNotExposeUpstreamDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("data: {\"error\":{\"message\":\"private upstream details\"}}\n\n"))
	}))
	defer server.Close()
	_, err := NewOpenAICompatible(server.URL, "key", "model", "").Stream(context.Background(), Request{}, func(StreamDelta) error { return nil })
	if err == nil || err.Error() != "provider returned a stream error" {
		t.Fatalf("unexpected stream error: %v", err)
	}
}
