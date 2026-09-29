package agentops

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/askxuan/ai-service/internal/askagent"
	"github.com/askxuan/ai-service/internal/knowledge"
)

func (m *Manager) bindReferences(in *askagent.Input, c Config, actor string) {
	if m.References == nil || actor == "" {
		return
	}
	in.KnowledgeEnabled = c.KnowledgeEnabled
	in.MemoryEnabled = c.MemoryEnabled
	in.References = func(ctx context.Context, name string) (string, error) {
		kind, owner := "knowledge", "platform"
		if name == "recall_memory" && in.MemoryEnabled {
			kind, owner = "memory", "admin:"+actor
		} else if name != "search_knowledge" || !in.KnowledgeEnabled {
			return "", errors.New("reference disabled")
		}
		var out knowledge.Result
		var e error
		if kind == "knowledge" {
			out, e = m.References.SearchKnowledge(ctx, in.Question, c.KnowledgeBaseIDs)
		} else {
			out, e = m.References.Search(ctx, kind, owner, in.Question)
		}
		if e != nil {
			return "", e
		}
		b, e := json.Marshal(out)
		return string(b), e
	}
}
