package einopoc

import (
	"context"
	"errors"

	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/settings"
	"github.com/cloudwego/eino/components/tool"
)

// FromSnapshot consumes one immutable settings snapshot, preserving the existing
// model allowlist and HTTP/DNS restrictions. It never changes admin settings.
func FromSnapshot(ctx context.Context, s *settings.Snapshot, modelID string, tools []tool.BaseTool, limits Limits) (*Harness, error) {
	if s == nil || s.Models == nil {
		return nil, errors.New("provider snapshot unavailable")
	}
	selected, err := s.Models.Select(ctx, modelID, false)
	if err != nil {
		return nil, err
	}
	maxTokens := s.Config.MaxOutputTokens
	if maxTokens < 1 || maxTokens > 512 {
		maxTokens = 512
	}
	chat, err := provider.NewEinoModel(ctx, s.Provider, provider.Request{Model: selected, MaxTokens: maxTokens, ThinkingEnabled: s.Config.ThinkingEnabled, ReasoningEffort: s.Config.ReasoningEffort})
	if err != nil {
		return nil, err
	}
	return New(ctx, chat, tools, agent.NewGuard(s.Config.MaxInputChars, s.Config.BlockedTerms), limits)
}
