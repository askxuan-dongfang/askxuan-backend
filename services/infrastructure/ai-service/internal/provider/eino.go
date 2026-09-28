package provider

import (
	"context"
	"errors"

	openai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
)

// NewEinoModel adapts an immutable provider snapshot without exporting its key or
// replacing its secure HTTP client. The experimental harness is the only caller;
// the production Complete/Stream path remains unchanged.
func NewEinoModel(ctx context.Context, source Provider, req Request) (model.BaseChatModel, error) {
	p, ok := source.(*OpenAICompatible)
	if !ok {
		return nil, errors.New("Eino requires an OpenAI-compatible provider")
	}
	if req.MaxTokens < 1 || req.MaxTokens > 2048 {
		return nil, errors.New("Eino probe output limit must be between 1 and 2048 tokens")
	}
	c := &openai.ChatModelConfig{
		APIKey: p.apiKey, BaseURL: p.baseURL, Model: p.ModelFor(req),
		HTTPClient: p.client, MaxTokens: &req.MaxTokens,
	}
	if req.ThinkingEnabled {
		c.ReasoningEffort = openai.ReasoningEffortLevel(req.ReasoningEffort)
	}
	if !p.standardParameters {
		mode := "disabled"
		if req.ThinkingEnabled {
			mode = "enabled"
		}
		c.ExtraFields = map[string]any{"thinking": map[string]string{"type": mode}}
	}
	return openai.NewChatModel(ctx, c)
}
