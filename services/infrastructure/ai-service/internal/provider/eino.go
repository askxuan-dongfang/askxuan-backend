package provider

import (
	"context"
	"errors"
	"net/http"

	openai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/google/uuid"
)

// NewEinoModel adapts an immutable provider snapshot without exporting its key or
// replacing its secure HTTP client. Both live tasks and admin evaluations use this adapter.
func NewEinoModel(ctx context.Context, source Provider, req Request) (model.BaseChatModel, error) {
	var err error
	source, req, err = ResolveProvider(ctx, source, req)
	if err != nil {
		return nil, err
	}
	p, ok := source.(*OpenAICompatible)
	if !ok {
		return nil, errors.New("Eino requires an OpenAI-compatible provider")
	}
	if req.MaxTokens < 1 || req.MaxTokens > 32768 {
		return nil, errors.New("Eino output limit must be between 1 and 32768 tokens")
	}
	c := &openai.ChatModelConfig{
		APIKey: p.apiKey, BaseURL: p.baseURL, Model: p.ModelFor(req),
		HTTPClient: p.client, MaxTokens: &req.MaxTokens,
	}
	if req.JSONMode {
		c.ResponseFormat = &openai.ChatCompletionResponseFormat{Type: openai.ChatCompletionResponseFormatTypeJSONObject}
	}
	if p.openCodeGo {
		if req.SessionID == "" {
			req.SessionID = "askxuan-" + uuid.NewString()
		}
		client := *p.client
		transport := client.Transport
		if transport == nil {
			transport = http.DefaultTransport
		}
		client.Transport = openCodeSessionTransport{base: transport, sessionID: req.SessionID}
		c.HTTPClient = &client
	}
	if req.ThinkingEnabled && !p.modelDefaultReasoning && !p.openCodeGo {
		c.ReasoningEffort = openai.ReasoningEffortLevel(req.ReasoningEffort)
		if p.Name() == "volcengine" {
			c.MaxTokens = nil
			c.MaxCompletionTokens = &req.MaxTokens
		}
	}
	if !p.standardParameters && !p.modelDefaultReasoning {
		mode := "disabled"
		if req.ThinkingEnabled {
			mode = "enabled"
		}
		c.ExtraFields = map[string]any{"thinking": map[string]string{"type": mode}}
	}
	return openai.NewChatModel(ctx, c)
}

type openCodeSessionTransport struct {
	base      http.RoundTripper
	sessionID string
}

func (t openCodeSessionTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	request := req.Clone(req.Context())
	setOpenCodeGoHeaders(request.Header, t.sessionID)
	return t.base.RoundTrip(request)
}

// ReasoningFallbackOptions permits one bounded non-thinking retry on adapters
// that explicitly support the thinking parameter. It never increases token limits.
func ReasoningFallbackOptions(source Provider, enabled bool, selected ...string) []model.Option {
	if r, ok := source.(*Registry); ok {
		id := r.Model()
		if len(selected) > 0 && selected[0] != "" {
			id = selected[0]
		}
		if route, found := r.routes[id]; found {
			source = route.Provider
		} else {
			source = r.primary
		}
	}
	p, ok := source.(*OpenAICompatible)
	if !enabled || !ok || p.standardParameters || p.modelDefaultReasoning {
		return nil
	}
	return []model.Option{openai.WithExtraFields(map[string]any{"thinking": map[string]string{"type": "disabled"}})}
}
