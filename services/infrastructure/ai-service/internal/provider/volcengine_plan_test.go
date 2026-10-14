package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

type planTransport func(*http.Request) (*http.Response, error)

func (f planTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestAgentPlanUsesModelDefaultsOnEveryPath(t *testing.T) {
	for _, path := range []string{"complete", "stream", "eino-generate", "eino-stream"} {
		for _, thinking := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/thinking=%v", path, thinking), func(t *testing.T) {
				p := newVolcengine(Config{BaseURL: VolcenginePlanBaseURL, APIKey: "fixture", Model: "ark-code-latest"})
				calls := 0
				p.SetHTTPClient(&http.Client{Transport: planTransport(func(r *http.Request) (*http.Response, error) {
					calls++
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Fatal(err)
					}
					if r.Method != "POST" || r.URL.String() != VolcenginePlanBaseURL+"/chat/completions" || r.Header.Get("Authorization") != "Bearer fixture" || body["model"] != "ark-code-latest" || body["max_tokens"] != float64(512) {
						t.Errorf("invalid Plan request: %s %+v", r.URL, body)
					}
					for _, key := range []string{"thinking", "reasoning_effort", "max_completion_tokens"} {
						if _, exists := body[key]; exists {
							t.Errorf("Plan must use model defaults; found %s", key)
						}
					}
					response := `{"model":"glm-fixture","choices":[{"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}],"usage":{"prompt_tokens":17,"completion_tokens":137,"completion_tokens_details":{"reasoning_tokens":132}}}`
					header := make(http.Header)
					header.Set("Content-Type", "application/json")
					if body["stream"] == true {
						header.Set("Content-Type", "text/event-stream")
						response = "data: {\"model\":\"glm-fixture\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"reasoning_content\":\"private reasoning\"}}]}\n\ndata: {\"model\":\"glm-fixture\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"OK\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":17,\"completion_tokens\":137,\"completion_tokens_details\":{\"reasoning_tokens\":132}}}\n\ndata: [DONE]\n\n"
					}
					return &http.Response{StatusCode: 200, Header: header, Body: io.NopCloser(strings.NewReader(response))}, nil
				})})
				if len(ReasoningFallbackOptions(p, true)) != 0 {
					t.Fatal("Plan must not retry with disabled thinking")
				}
				if _, err := NewCatalog(p).List(context.Background()); err != nil || calls != 0 {
					t.Fatal("Plan catalog must not query /models", err)
				}
				req := Request{Messages: []Message{{Role: "user", Content: "Reply with OK only."}}, MaxTokens: 512, ThinkingEnabled: thinking, ReasoningEffort: "high"}
				ctx := context.Background()
				var content strings.Builder
				if strings.HasPrefix(path, "eino-") {
					chat, err := NewEinoModel(ctx, p, req)
					if err != nil {
						t.Fatal(err)
					}
					messages := []*schema.Message{schema.UserMessage("Reply with OK only.")}
					if path == "eino-generate" {
						out, err := chat.Generate(ctx, messages)
						if err != nil {
							t.Fatal(err)
						}
						content.WriteString(out.Content)
					} else {
						stream, err := chat.Stream(ctx, messages)
						if err != nil {
							t.Fatal(err)
						}
						defer stream.Close()
						for {
							out, err := stream.Recv()
							if errors.Is(err, io.EOF) {
								break
							}
							if err != nil {
								t.Fatal(err)
							}
							content.WriteString(out.Content)
						}
					}
				} else {
					var out *Response
					var err error
					if path == "complete" {
						out, err = p.Complete(ctx, req)
					} else {
						out, err = p.Stream(ctx, req, func(d StreamDelta) error { content.WriteString(d.Content); return nil })
					}
					if err != nil {
						t.Fatal(err)
					}
					if out.Model != "glm-fixture" || out.TotalTokens() != 154 || out.ReasoningTokens != 132 {
						t.Fatalf("lost model/usage: %+v", out)
					}
					if path == "complete" {
						content.WriteString(out.Content)
					}
				}
				if content.String() != "OK" || calls != 1 {
					t.Fatalf("invalid output/calls: %q %d", content.String(), calls)
				}
			})
		}
	}
}
