package logic

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/askxuan/ai-service/internal/askagent"
	"github.com/askxuan/ai-service/internal/knowledge"
	"github.com/askxuan/ai-service/internal/svc"
)

func bindReferences(s *svc.ServiceContext, in *askagent.Input, user string, observe func(string, string)) {
	if s.Knowledge == nil || user == "" {
		return
	}
	in.KnowledgeEnabled = s.KnowledgeEnabled
	in.MemoryEnabled = s.MemoryEnabled
	in.References = func(ctx context.Context, name, query string) (string, error) {
		kind, owner := "knowledge", "platform"
		if name == "recall_memory" && in.MemoryEnabled {
			kind, owner = "memory", user
		} else if name != "search_knowledge" || !in.KnowledgeEnabled {
			return "", errors.New("reference access disabled")
		}
		var r knowledge.Result
		var e error
		if kind == "knowledge" {
			if query == "" {
				query = in.Question
			}
			r, e = s.Knowledge.SearchKnowledge(ctx, query, s.KnowledgeBases)
		} else {
			r, e = s.Knowledge.Search(ctx, kind, owner, in.Question)
		}
		if e != nil {
			return "", e
		}
		b, e := json.Marshal(r)
		if e == nil && observe != nil {
			observe(name, string(b))
		}
		return string(b), e
	}
}
