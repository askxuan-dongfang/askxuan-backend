package logic

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"sync"

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
	var mu sync.Mutex
	numbers := map[string]int{}
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
			r, e = s.Knowledge.SearchKnowledge(knowledge.WithRetrieval(ctx, s.Retrieval), query, s.KnowledgeBases)
		} else {
			r, e = s.Knowledge.Search(ctx, kind, owner, in.Question)
		}
		if e != nil {
			return "", e
		}
		if kind == "knowledge" {
			// Keep source numbers stable across retries and parallel searches in
			// this answer. Private memory never becomes a public citation.
			mu.Lock()
			for i := range r.Hits {
				key := r.Hits[i].ID + "\x00" + r.Hits[i].Hash + "\x00" + strconv.FormatInt(r.Hits[i].Revision, 10)
				if numbers[key] == 0 {
					numbers[key] = len(numbers) + 1
				}
				r.Hits[i].Citation = numbers[key]
			}
			mu.Unlock()
		}
		b, e := json.Marshal(r)
		if e == nil && observe != nil {
			observe(name, string(b))
		}
		if e == nil && kind == "knowledge" && in.Report != nil && in.Report.Calculated && len(r.Graph) > 0 {
			// Report claims use cited passages; retain the full graph only in the private audit.
			r.Graph = nil
			b, e = json.Marshal(r)
		}
		return string(b), e
	}
}

// These are retrieval snapshots, not model-written references or an assertion
// that every retrieved passage supports the answer. The UI labels them as such.
type referenceSnapshots struct {
	mu   sync.Mutex
	hits []knowledge.Hit
}

func (s *referenceSnapshots) snapshot() []knowledge.Hit {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]knowledge.Hit(nil), s.hits...)
}

func (s *referenceSnapshots) observe(name, raw string) {
	if name != "search_knowledge" {
		return
	}
	var result knowledge.Result
	if json.Unmarshal([]byte(raw), &result) != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, hit := range result.Hits {
		if hit.Citation < 1 || hit.ID == "" || hit.Text == "" || hit.Source == "" || len(s.hits) >= 48 {
			continue
		}
		duplicate := false
		for _, saved := range s.hits {
			if saved.Citation == hit.Citation {
				duplicate = true
				break
			}
		}
		if !duplicate {
			s.hits = append(s.hits, hit)
		}
	}
}
