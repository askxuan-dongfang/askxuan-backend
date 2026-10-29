package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

type einoJSONModeTransport func(*http.Request) (*http.Response, error)

func (f einoJSONModeTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestEinoJSONModePreservesToolRoundTripAndDefaultChat(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			calls := 0
			p := NewOpenAICompatible("http://eino-json-mode.invalid", "fixture", "fixture-model", "")
			p.SetHTTPClient(&http.Client{Transport: einoJSONModeTransport(func(r *http.Request) (*http.Response, error) {
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					return nil, err
				}
				call := calls
				calls++
				format, present := payload["response_format"]
				if enabled && call < 2 {
					object, _ := format.(map[string]any)
					if len(object) != 1 || object["type"] != "json_object" {
						t.Errorf("report step %d lost JSON mode: %v", call, format)
					}
				} else if present {
					t.Errorf("default chat gained response_format: %v", format)
				}
				thinking, _ := payload["thinking"].(map[string]any)
				if payload["model"] != "fixture-model" || payload["max_tokens"] != float64(512) || thinking["type"] != "enabled" || payload["reasoning_effort"] != "low" {
					t.Error("JSON mode changed model, budget or reasoning policy")
				}
				header := make(http.Header)
				if call == 2 {
					if _, present := payload["tools"]; present {
						t.Error("tool binding leaked to a new ordinary chat model")
					}
					header.Set("Content-Type", "application/json")
					return &http.Response{StatusCode: 200, Header: header, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"role":"assistant","content":"plain chat"},"finish_reason":"stop"}]}`))}, nil
				}
				tools, _ := payload["tools"].([]any)
				if len(tools) != 1 {
					t.Fatal("JSON mode removed the tool definition")
				}
				function, _ := tools[0].(map[string]any)["function"].(map[string]any)
				if function["name"] != "read_report" || payload["tool_choice"] != "auto" || payload["stream"] != true {
					t.Error("streaming tool request changed")
				}
				header.Set("Content-Type", "text/event-stream")
				var event any
				if call == 0 {
					event = map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": "call-read", "type": "function", "function": map[string]string{"name": "read_report", "arguments": "{}"}}}}, "finish_reason": "tool_calls"}}}
				} else {
					messages := payload["messages"].([]any)
					last := messages[len(messages)-1].(map[string]any)
					if last["role"] != "tool" || last["tool_call_id"] != "call-read" || last["content"] != `{"chart":"synthetic"}` {
						t.Error("tool result was not preserved before the JSON answer")
					}
					event = map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]string{"role": "assistant", "content": `{"summary":"synthetic summary","content":"synthetic report"}`}, "finish_reason": "stop"}}}
				}
				raw, _ := json.Marshal(event)
				return &http.Response{StatusCode: 200, Header: header, Body: io.NopCloser(strings.NewReader("data: " + string(raw) + "\n\ndata: [DONE]\n\n"))}, nil
			})})
			ctx := context.Background()
			request := Request{JSONMode: enabled, MaxTokens: 512, ThinkingEnabled: true, ReasoningEffort: "low"}
			chat, err := NewEinoModel(ctx, p, request)
			if err != nil {
				t.Fatal(err)
			}
			toolModel, ok := chat.(model.ToolCallingChatModel)
			if !ok {
				t.Fatal("adapter lost tool-calling support")
			}
			bound, err := toolModel.WithTools([]*schema.ToolInfo{{Name: "read_report", ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{})}})
			if err != nil {
				t.Fatal(err)
			}
			stream := func(messages []*schema.Message) *schema.Message {
				t.Helper()
				reader, err := bound.Stream(ctx, messages)
				if err != nil {
					t.Fatal(err)
				}
				defer reader.Close()
				var chunks []*schema.Message
				for {
					chunk, err := reader.Recv()
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Fatal(err)
					}
					chunks = append(chunks, chunk)
				}
				answer, err := schema.ConcatMessages(chunks)
				if err != nil {
					t.Fatal(err)
				}
				return answer
			}
			messages := []*schema.Message{schema.SystemMessage("Return the final answer as JSON after reading the report."), schema.UserMessage("Read the synthetic chart.")}
			call := stream(messages)
			if len(call.ToolCalls) != 1 || call.ToolCalls[0].ID != "call-read" || call.ToolCalls[0].Function.Name != "read_report" || call.ToolCalls[0].Function.Arguments != "{}" {
				t.Fatalf("tool call was not decoded: %#v", call.ToolCalls)
			}
			messages = append(messages, call, &schema.Message{Role: schema.Tool, ToolCallID: "call-read", Content: `{"chart":"synthetic"}`})
			answer := stream(messages)
			var body map[string]string
			if err := json.Unmarshal([]byte(answer.Content), &body); err != nil || body["content"] != "synthetic report" || len(answer.ToolCalls) != 0 {
				t.Fatalf("final JSON answer lost: %q %v", answer.Content, err)
			}
			request.JSONMode = false
			ordinary, err := NewEinoModel(ctx, p, request)
			if err != nil {
				t.Fatal(err)
			}
			plain, err := ordinary.Generate(ctx, []*schema.Message{schema.UserMessage("ordinary chat")})
			if err != nil || plain.Content != "plain chat" || calls != 3 {
				t.Fatalf("ordinary chat changed: %#v %v calls=%d", plain, err, calls)
			}
		})
	}
}
