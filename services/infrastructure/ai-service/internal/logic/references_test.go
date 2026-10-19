package logic

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/askxuan/ai-service/internal/askagent"
	"github.com/askxuan/ai-service/internal/knowledge"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/svc"
)

type referenceSearch struct{}

func (referenceSearch) Search(_ context.Context, query string, _ []string) (knowledge.Result, error) {
	a := knowledge.Hit{ID: "a", Title: "原文甲", Text: "甲的原文", Source: "https://example.org/a", Hash: "hash-a", Revision: 2}
	b := knowledge.Hit{ID: "b", Title: "原文乙", Text: "乙的原文", Source: "https://example.org/b", Hash: "hash-b", Revision: 1}
	if query == "revised" {
		a.Revision = 3
		return knowledge.Result{Hits: []knowledge.Hit{a}}, nil
	}
	if query == "second" {
		return knowledge.Result{Hits: []knowledge.Hit{b, a}}, nil
	}
	return knowledge.Result{Hits: []knowledge.Hit{a}}, nil
}

func TestKnowledgeReferencesSurviveRepeatSearchAndMessageSerialization(t *testing.T) {
	ctx := context.Background()
	in := askagent.Input{}
	s := &svc.ServiceContext{Knowledge: &knowledge.Store{Remote: referenceSearch{}}, KnowledgeEnabled: true}
	snapshots := &referenceSnapshots{}
	bindReferences(s, &in, "owner", snapshots.observe)
	first, err := in.References(ctx, "search_knowledge", "first")
	if err != nil {
		t.Fatal(err)
	}
	second, err := in.References(ctx, "search_knowledge", "second")
	if err != nil {
		t.Fatal(err)
	}
	var a, b knowledge.Result
	_ = json.Unmarshal([]byte(first), &a)
	_ = json.Unmarshal([]byte(second), &b)
	if a.Hits[0].Citation != 1 || b.Hits[0].Citation != 2 || b.Hits[1].Citation != 1 {
		t.Fatal("source numbers changed between searches")
	}
	// Even a memory result shaped like a public reference must not be retained.
	snapshots.observe("recall_memory", `{"hits":[{"citation":3,"id":"private","text":"私人记忆","source":"private"}]}`)
	refs := snapshots.snapshot()
	if len(refs) != 2 || refs[0].Text != "甲的原文" || refs[0].Hash != "hash-a" || refs[1].Title != "原文乙" {
		t.Fatal("lost source evidence, duplicated reference, or exposed memory")
	}
	raw, _ := json.Marshal(map[string]any{"_agent": map[string]any{"runtime": "harness", "references": refs}})
	message := toTypesMessage(model.AIMessage{Role: "assistant", InputJSON: string(raw)})
	var persisted []knowledge.Hit
	if message.Agent == nil || json.Unmarshal(message.Agent.References, &persisted) != nil || len(persisted) != 2 || persisted[1].Citation != 2 {
		t.Fatal("customer response lost the persisted sources")
	}
	if _, err = in.References(ctx, "recall_memory", "private"); err == nil {
		t.Fatal("disabled private memory was accessible")
	}
}

func TestKnowledgeReferenceRevisionHasDistinctProvenance(t *testing.T) {
	in := askagent.Input{}
	service := &svc.ServiceContext{Knowledge: &knowledge.Store{Remote: referenceSearch{}}, KnowledgeEnabled: true}
	snapshots := &referenceSnapshots{}
	bindReferences(service, &in, "owner", snapshots.observe)
	for _, query := range []string{"first", "revised"} {
		if _, err := in.References(context.Background(), "search_knowledge", query); err != nil {
			t.Fatal(err)
		}
	}
	refs := snapshots.snapshot()
	if len(refs) != 2 || refs[0].Citation == refs[1].Citation || refs[0].Revision != 2 || refs[1].Revision != 3 {
		t.Fatal("a revised source was incorrectly attributed to an earlier revision")
	}
}
