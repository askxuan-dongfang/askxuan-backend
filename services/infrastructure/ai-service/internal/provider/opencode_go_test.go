package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

type openCodeTestTransport func(*http.Request) (*http.Response, error)

func (f openCodeTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestOpenCodeGoDiscoveryChecksAuthAndTextCapabilities(t *testing.T) {
	status, body := 200, `{"object":"list","data":[
		{"type":"chat","id":"kimi-k3","name":"Kimi K3","limit":{"context":2097152,"output":1048576},"modalities":{"input":["text","image/png"],"output":["text"]}},
		{"type":"chat","id":"short-text"},
		{"type":"embedding","id":"embed"},
		{"type":"chat","id":"images","modalities":{"input":["image/png"],"output":["image/png"]}},
		{"id":"unverified"},
		{"type":"chat","id":"kimi-k3"}
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

func TestOpenCodeGoChatRoutesBothRuntimesThroughTheSelectedModel(t *testing.T) {
	ctx := context.Background()
	p, err := New(Config{Provider: OpenCodeGo, BaseURL: OpenCodeGoBaseURL, Model: "kimi-k3", APIKey: "fixture-key"})
	if err != nil {
		t.Fatal(err)
	}
	const session = "askxuan-chat-fixture"
	calls := 0
	p.(*OpenAICompatible).SetHTTPClient(&http.Client{Transport: openCodeTestTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != OpenCodeGoBaseURL+"/chat/completions" || r.UserAgent() != openCodeUserAgent ||
			r.Header.Get("x-opencode-session") != session || r.Header.Get("Authorization") != "Bearer fixture-key" {
			t.Fatalf("wrong request route or headers for chat request %d", calls)
		}
		var body struct {
			Model     string          `json:"model"`
			Stream    bool            `json:"stream"`
			Thinking  json.RawMessage `json:"thinking"`
			Reasoning json.RawMessage `json:"reasoning_effort"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Model != "kimi-k3" || body.Thinking != nil || body.Reasoning != nil {
			t.Fatalf("wrong chat request: %+v %v", body, err)
		}
		header := http.Header{"Content-Type": {"application/json"}}
		text := `{"model":"kimi-k3","choices":[{"index":0,"message":{"role":"assistant","content":"Ready"},"finish_reason":"stop"}]}`
		if body.Stream {
			header.Set("Content-Type", "text/event-stream")
			text = "data: " + `{"model":"kimi-k3","choices":[{"index":0,"delta":{"role":"assistant","content":"Ready"},"finish_reason":"stop"}]}` +
				"\n\ndata: [DONE]\n\n"
		}
		return &http.Response{StatusCode: 200, Header: header, Body: io.NopCloser(strings.NewReader(text))}, nil
	})})
	registry := NewRegistry(Mock{}, nil, []RegisteredRoute{{Option: ModelOption{ID: "stored-go"}, Provider: p, Model: "kimi-k3"}}, "stored-go", "")
	req := Request{SessionID: session, MaxTokens: 128, ThinkingEnabled: true, ReasoningEffort: "high",
		Messages: []Message{{Role: "user", Content: "Say Ready"}}}
	result, err := registry.Stream(ctx, req, func(StreamDelta) error { return nil })
	if err != nil || result.Content != "Ready" {
		t.Fatalf("classic stream: %+v, %v", result, err)
	}

	chat, err := NewEinoModel(ctx, registry, req)
	if err != nil {
		t.Fatal(err)
	}
	input := []*schema.Message{schema.UserMessage("Say Ready")}
	reply, err := chat.Generate(ctx, input)
	if err != nil || reply.Content != "Ready" {
		t.Fatalf("agent completion: %+v, %v", reply, err)
	}
	stream, err := chat.Stream(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	delta, err := stream.Recv()
	if err != nil || delta.Content != "Ready" || calls != 3 {
		t.Fatalf("agent stream: %+v, %v (%d requests)", delta, err, calls)
	}
}
