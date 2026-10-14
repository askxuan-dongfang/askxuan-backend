package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestVolcengineCatalogUsesConfiguredIDs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("registered Ark models must not call /models")
		http.NotFound(w, r)
	}))
	defer server.Close()
	p, err := New(Config{Provider: "volcengine", BaseURL: server.URL, APIKey: "fixture", Model: "ep-20261014-text", VisionModel: "ep-20261014-vision", EnabledModels: []string{"ep-20261014-text", "doubao-fixture"}})
	if err != nil || p.Name() != "volcengine" {
		t.Fatalf("factory: %v %v", p, err)
	}
	c := NewCatalog(p)
	list, err := c.List(context.Background())
	if err != nil || len(list.List) != 3 || list.DefaultModel != "ep-20261014-text" || list.Stale {
		t.Fatalf("configured catalog: %+v %v", list, err)
	}
	if _, err = c.Select(context.Background(), "ep-20261014-vision", true); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Select(context.Background(), "ep-20261014-text", true); !errors.Is(err, ErrModelNeedsVision) {
		t.Fatalf("text model accepted images: %v", err)
	}
	filtered := NewCatalogWithAllowed(p, []string{"ep-20261014-text"})
	if _, err = filtered.Select(context.Background(), "doubao-fixture", false); !errors.Is(err, ErrModelUnavailable) {
		t.Fatalf("hidden model accepted: %v", err)
	}
	window, output, err := c.Limits(context.Background(), "ep-20261014-text", 1048576, 20000)
	if err != nil || window != 32768 || output != 16384 {
		t.Fatalf("unknown endpoint metadata must be conservative: %d %d %v", window, output, err)
	}
	list.List[0].ID = "mutated"
	again, _ := c.List(context.Background())
	if again.List[0].ID == "mutated" {
		t.Fatal("catalog exposed mutable snapshot")
	}
}

func TestVolcengineConfigAndOfficialEndpoints(t *testing.T) {
	p, err := New(Config{Provider: "volcengine", APIKey: "fixture", Model: "ep-fixture"})
	if err != nil || p.(*OpenAICompatible).baseURL != VolcengineBaseURL {
		t.Fatal("missing default Ark address", err)
	}
	for _, cfg := range []Config{{Provider: "volcengine"}, {Provider: "volcengine", APIKey: "fixture"}, {Provider: "volcengine", Model: "ep-fixture"}} {
		if _, err := New(cfg); err == nil {
			t.Fatal("missing key/model accepted")
		}
	}
	for _, raw := range []string{VolcengineBaseURL, "https://ark.cn-beijing.volces.com:443/api/v3/"} {
		if !IsVolcengineURL(raw) {
			t.Fatal("official endpoint rejected", raw)
		}
	}
	for _, raw := range []string{"https://ark.cn-beijing.volces.com.attacker.test/api/v3", "http://ark.cn-beijing.volces.com/api/v3", "https://ark.cn-beijing.volces.com/api/coding/v3", "https://ark.cn-beijing.volces.com/api/v3/chat/completions", "https://ark.cn-beijing.volces.com:8443/api/v3", "https://user@ark.cn-beijing.volces.com/api/v3", VolcengineBaseURL + "?key=secret"} {
		if IsVolcengineURL(raw) {
			t.Fatal("invalid endpoint accepted", raw)
		}
	}
}

func TestVolcengineCompleteStreamAndEinoWireContract(t *testing.T) {
	for _, path := range []string{"complete", "stream", "eino"} {
		for _, thinking := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/thinking=%v", path, thinking), func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var body struct {
						Model               string `json:"model"`
						MaxTokens           int    `json:"max_tokens"`
						MaxCompletionTokens int    `json:"max_completion_tokens"`
						Thinking            struct {
							Type string `json:"type"`
						} `json:"thinking"`
						Reasoning string `json:"reasoning_effort"`
						Stream    bool   `json:"stream"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					mode := "disabled"
					if thinking {
						mode = "enabled"
					}
					if r.URL.Path != "/api/v3/chat/completions" || r.Header.Get("Authorization") != "Bearer fixture-key" || body.Model != "ep-fixture" || body.Thinking.Type != mode || (thinking && (body.Reasoning != "low" || body.MaxCompletionTokens != 128 || body.MaxTokens != 0)) || (!thinking && (body.Reasoning != "" || body.MaxTokens != 128 || body.MaxCompletionTokens != 0)) {
						t.Errorf("bad Ark request: %s %+v", r.URL.Path, body)
					}
					if body.Stream {
						w.Header().Set("Content-Type", "text/event-stream")
						fmt.Fprint(w, "data: {\"model\":\"doubao-fixture\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"OK\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":1}}\n\ndata: [DONE]\n\n")
					} else {
						w.Header().Set("Content-Type", "application/json")
						fmt.Fprint(w, `{"model":"doubao-fixture","choices":[{"message":{"role":"assistant","content":"OK"}}],"usage":{"prompt_tokens":3,"completion_tokens":1}}`)
					}
				}))
				defer server.Close()
				p, err := New(Config{Provider: "volcengine", BaseURL: server.URL + "/api/v3", APIKey: "fixture-key", Model: "ep-fixture"})
				if err != nil {
					t.Fatal(err)
				}
				req := Request{Messages: []Message{{Role: "user", Content: "hello"}}, MaxTokens: 128, ThinkingEnabled: thinking, ReasoningEffort: "low"}
				if path == "eino" {
					chat, err := NewEinoModel(context.Background(), p, req)
					if err != nil {
						t.Fatal(err)
					}
					stream, err := chat.Stream(context.Background(), []*schema.Message{schema.UserMessage("hello")})
					if err != nil {
						t.Fatal(err)
					}
					defer stream.Close()
					var text strings.Builder
					for {
						msg, err := stream.Recv()
						if errors.Is(err, io.EOF) {
							break
						}
						if err != nil {
							t.Fatal(err)
						}
						text.WriteString(msg.Content)
					}
					if text.String() != "OK" {
						t.Fatal("Eino lost streamed content", text.String())
					}
					return
				}
				var out *Response
				if path == "complete" {
					out, err = p.Complete(context.Background(), req)
				} else {
					out, err = p.Stream(context.Background(), req, func(StreamDelta) error { return nil })
				}
				if err != nil || out.Content != "OK" || out.TotalTokens() != 4 || out.Model != "doubao-fixture" {
					t.Fatalf("response: %+v %v", out, err)
				}
			})
		}
	}
}
