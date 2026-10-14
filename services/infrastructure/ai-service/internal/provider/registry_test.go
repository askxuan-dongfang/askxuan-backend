package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestRegistryKeepsCredentialsAndWireModelsIsolated(t *testing.T) {
	var calls atomic.Int32
	makeRoute := func(id, key string, plan bool) RegisteredRoute {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			if r.URL.Path != "/chat/completions" || r.Header.Get("Authorization") != "Bearer "+key {
				t.Error("connection crossed routes")
			}
			var body map[string]any
			if json.NewDecoder(r.Body).Decode(&body) != nil || body["model"] != "same-wire-model" {
				t.Error("platform identity sent upstream")
			}
			if plan && (body["thinking"] != nil || body["reasoning_effort"] != nil) {
				t.Error("Plan received unsupported reasoning override")
			}
			if body["stream"] == true {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprintf(w, "data: {\"model\":\"same-wire-model\",\"choices\":[{\"delta\":{\"content\":\"%s\"},\"finish_reason\":null}]}\n\ndata: [DONE]\n\n", id)
			} else {
				json.NewEncoder(w).Encode(map[string]any{"model": "same-wire-model", "choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": id}, "finish_reason": "stop"}}})
			}
		}))
		t.Cleanup(srv.Close)
		p := NewOpenAICompatible(srv.URL, key, "same-wire-model", "")
		p.modelDefaultReasoning = plan
		return RegisteredRoute{Option: ModelOption{ID: id, Name: id, ContextWindow: 256000, MaxOutputTokens: 32000}, Provider: p, Model: "same-wire-model"}
	}
	a, b := makeRoute("registered-a", "fixture-a", true), makeRoute("registered-b", "fixture-b", false)
	r := NewRegistry(Mock{}, nil, []RegisteredRoute{a, b}, a.Option.ID, "")
	ctx := context.Background()
	var wg sync.WaitGroup
	for _, id := range []string{a.Option.ID, b.Option.ID} {
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				out, err := r.Complete(ctx, Request{Model: id, Messages: []Message{{Role: "user", Content: "hello"}}, MaxTokens: 128})
				if err != nil || out.Content != id {
					t.Errorf("route %s: %v %v", id, out, err)
				}
			}(id)
		}
	}
	wg.Wait()
	chat, err := NewEinoModel(ctx, r, Request{Model: a.Option.ID, MaxTokens: 128, ThinkingEnabled: true, ReasoningEffort: "high"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := chat.Generate(ctx, []*schema.Message{schema.UserMessage("hello")})
	if err != nil || out.Content != a.Option.ID {
		t.Fatal(out, err)
	}
	var streamed string
	_, err = r.Stream(ctx, Request{Model: b.Option.ID, MaxTokens: 128, Messages: []Message{{Role: "user", Content: "hello"}}}, func(d StreamDelta) error { streamed += d.Content; return nil })
	if err != nil || streamed != b.Option.ID {
		t.Fatal(streamed, err)
	}
	before := calls.Load()
	if _, err = r.Complete(ctx, Request{Model: "unregistered"}); !errors.Is(err, ErrModelUnavailable) || calls.Load() != before {
		t.Fatal("unknown model was dispatched", err)
	}
	if len(ReasoningFallbackOptions(r, true, a.Option.ID)) != 0 || len(ReasoningFallbackOptions(r, true, b.Option.ID)) == 0 {
		t.Fatal("fallback used wrong provider")
	}
	catalog := NewCatalog(r)
	window, cap, err := catalog.Limits(ctx, a.Option.ID, 1048576, 32768)
	if err != nil || window != 256000 || cap != 32000 {
		t.Fatal(window, cap, err)
	}
	if r.Model() != a.Option.ID || a.Provider.Model() != "same-wire-model" {
		t.Fatal("shared defaults mutated")
	}
}
