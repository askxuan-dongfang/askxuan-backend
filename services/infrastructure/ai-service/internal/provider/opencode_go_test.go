package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type openCodeTestTransport func(*http.Request) (*http.Response, error)

func (f openCodeTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOpenCodeGoDiscoveryChecksAuthAndTextCapabilities(t *testing.T) {
	status, body := 200, `{"object":"list","data":[
		{"type":"chat","id":"kimi-k3","name":"Kimi K3","limit":{"context":2097152,"output":1048576},"modalities":{"input":["text","image/png"],"output":["text"]},
		 "api":{"chat-completions":{"base_url":"https://opencode.ai/inference/go/openai/v1"}}},
		{"type":"chat","id":"short-text","api":{"chat-completions":{"base_url":"https://opencode.ai/inference/go/openai/v1/"}}},
		{"type":"embedding","id":"embed"},
		{"type":"chat","id":"images","modalities":{"input":["image/png"],"output":["image/png"]},
		 "api":{"chat-completions":{"base_url":"https://opencode.ai/inference/go/openai/v1"}}},
		{"type":"chat","id":"messages-only","modalities":{"input":["text"],"output":["text"]},
		 "api":{"messages":{"base_url":"https://opencode.ai/inference/go/anthropic/v1"}}},
		{"type":"chat","id":"unknown-api"},
		{"type":"chat","id":"off-host","api":{"chat-completions":{"base_url":"https://elsewhere.invalid/v1"}}},
		{"id":"unverified"},
		{"type":"chat","id":"kimi-k3","api":{"chat-completions":{"base_url":"https://opencode.ai/inference/go/openai/v1"}}}
	]}`
	client := &http.Client{Transport: openCodeTestTransport(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet || r.URL.String() != openCodeGoModels ||
			r.Header.Get("Authorization") != "Bearer fixture-key" || r.UserAgent() != openCodeUserAgent {
			t.Fatalf("invalid discovery route/metadata: %s", r.URL)
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}
	options, err := DiscoverOpenCodeGo(context.Background(), client, "fixture-key")
	if err != nil || len(options) != 2 || options[0].Name != "Kimi K3" || options[0].ContextWindow != 2097152 ||
		options[0].MaxOutputTokens != 1048576 || !options[0].SupportsVision || options[1].MaxOutputTokens != 0 {
		t.Fatalf("incorrect Go options: %+v %v", options, err)
	}
	body = `{"object":"list","data":[],"error":{"message":"private upstream value"}}`
	if _, err := DiscoverOpenCodeGo(context.Background(), client, "fixture-key"); err == nil || strings.Contains(err.Error(), "private") {
		t.Fatalf("provider business error mishandled: %v", err)
	}
	status = 403
	var upstream *HTTPError
	if _, err := DiscoverOpenCodeGo(context.Background(), client, "fixture-key"); !errors.As(err, &upstream) || upstream.StatusCode != 403 {
		t.Fatalf("provider HTTP error mishandled: %v", err)
	}
}

func TestOpenCodeGoUsesFixedChatEndpointAndOwnSessionHeaders(t *testing.T) {
	cfg := Config{Provider: OpenCodeGo, BaseURL: OpenCodeGoBaseURL, Model: "kimi-k3", APIKey: "fixture-key"}
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	p.(*OpenAICompatible).SetHTTPClient(&http.Client{Transport: openCodeTestTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != http.MethodPost || r.URL.String() != OpenCodeGoBaseURL+"/chat/completions" ||
			r.Header.Get("Authorization") != "Bearer fixture-key" || r.UserAgent() != openCodeUserAgent ||
			r.Header.Get("x-opencode-session") != "askxuan-catalog-fixture" {
			t.Fatalf("unsupported Go request route/metadata: %s", r.URL)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		format, _ := body["response_format"].(map[string]any)
		if body["model"] != "kimi-k3" || body["max_tokens"] != float64(768) || body["thinking"] != nil ||
			body["reasoning_effort"] != nil || format["type"] != "json_object" {
			t.Fatalf("unsupported Go request shape: %+v", body)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(
			`{"choices":[{"message":{"content":"{\"title\":\"Cloud Beads\"}"},"finish_reason":"stop"}]}`))}, nil
	})})
	req := Request{MaxTokens: 768, JSONMode: true, ThinkingEnabled: true, ReasoningEffort: "low",
		Messages: []Message{{Role: "user", Content: "Return JSON."}}}
	if _, err := p.Complete(context.Background(), req); err == nil || calls != 0 {
		t.Fatal("missing session was sent to Go")
	}
	req.SessionID = "askxuan-catalog-fixture"
	if reply, err := p.Complete(context.Background(), req); err != nil || !strings.Contains(reply.Content, "Cloud Beads") || calls != 1 {
		t.Fatalf("completion: %+v %v", reply, err)
	}
	cfg.BaseURL = "https://unrelated.example/v1"
	if _, err := New(cfg); err == nil {
		t.Fatal("Go credential could be used at another host")
	}
}
