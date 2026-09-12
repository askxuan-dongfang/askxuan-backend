package provider

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCatalogConcurrentCacheAndCapabilities(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/models" || r.Header.Get("Authorization") != "Bearer test-only" {
			t.Error("incorrect catalog authentication")
		}
		w.Write([]byte(`{"data":[{"id":"deepseek-v4-pro"},{"id":"deepseek-flash"},{"id":"deepseek-flash"},{"id":"../invalid"},{"id":"future-text-model"}]}`))
	}))
	defer server.Close()
	catalog := NewCatalog(NewOpenAICompatible(server.URL, "test-only", "deepseek-flash", "deepseek-flash"))
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			list, err := catalog.List(context.Background())
			if err != nil || len(list.List) != 3 || list.DefaultModel != "deepseek-flash" {
				t.Errorf("catalog: %+v %v", list, err)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("expected one fetch, got %d", calls.Load())
	}
	if _, err := catalog.Select(context.Background(), "arbitrary-model", false); !errors.Is(err, ErrModelUnavailable) {
		t.Fatal(err)
	}
	if _, err := catalog.Select(context.Background(), "deepseek-v4-pro", true); !errors.Is(err, ErrModelNeedsVision) {
		t.Fatal(err)
	}
	if id, err := catalog.Select(context.Background(), "deepseek-flash", true); err != nil || id != "deepseek-flash" {
		t.Fatal(id, err)
	}
	// Callers cannot mutate the shared cached result.
	list, _ := catalog.List(context.Background())
	list.List[0].ID = "corrupted"
	list, _ = catalog.List(context.Background())
	if list.List[0].ID == "corrupted" {
		t.Fatal("mutable cache")
	}
}

func TestCatalogOutageAndExpiryDoNotExposeProviderBody(t *testing.T) {
	fail := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			w.WriteHeader(401)
			w.Write([]byte("sensitive provider diagnostic"))
			return
		}
		w.Write([]byte(`{"data":[{"id":"deepseek-flash"}]}`))
	}))
	defer server.Close()
	c := NewCatalog(NewOpenAICompatible(server.URL, "test-only", "deepseek-flash", ""))
	ctx := context.Background()
	if _, err := c.List(ctx); err != nil {
		t.Fatal(err)
	}
	fail = true
	c.fetched = time.Now().Add(-6 * time.Minute)
	c.attempted = time.Time{}
	list, err := c.List(ctx)
	if err != nil || !list.Stale {
		t.Fatal(list, err)
	}
	c.fetched = time.Now().Add(-2 * time.Hour)
	if _, err = c.List(ctx); err == nil || strings.Contains(err.Error(), "sensitive") {
		t.Fatal("expected sanitized failure", err)
	}
}

func TestPerRequestModelDoesNotChangeSharedProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		model := body["model"].(string)
		json.NewEncoder(w).Encode(map[string]interface{}{"model": model, "choices": []map[string]interface{}{{"message": map[string]string{"role": "assistant", "content": model}}}})
	}))
	defer server.Close()
	p := NewOpenAICompatible(server.URL, "test-only", "deepseek-flash", "deepseek-flash")
	var wg sync.WaitGroup
	for _, id := range []string{"deepseek-v4-pro", "deepseek-flash"} {
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				result, err := p.Complete(context.Background(), Request{Model: id, Messages: []Message{{Role: "user", Content: "isolated"}}})
				if err != nil || result.Model != id {
					t.Errorf("model crossover: %v %v", result, err)
				}
			}(id)
		}
	}
	wg.Wait()
	if p.Model() != "deepseek-flash" {
		t.Fatal("provider default was mutated")
	}
}

func TestStandardProviderDoesNotSendDeepSeekOnlyParameters(t *testing.T) {
	p := NewOpenAICompatible("https://example.com/v1", "fixture-key", "example-model", "")
	p.UseStandardParameters()
	payload := p.payload(Request{ThinkingEnabled: true, ReasoningEffort: "high"}, "example-model", nil)
	if _, ok := payload["thinking"]; ok {
		t.Fatal("DeepSeek-only thinking parameter sent to another provider")
	}
	if payload["reasoning_effort"] != "high" {
		t.Fatal("standard reasoning parameter missing")
	}
}
