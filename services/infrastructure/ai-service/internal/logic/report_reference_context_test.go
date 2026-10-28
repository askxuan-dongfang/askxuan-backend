package logic

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/askxuan/ai-service/internal/askagent"
	"github.com/askxuan/ai-service/internal/knowledge"
	"github.com/askxuan/ai-service/internal/svc"
)

type graphReferenceSearch struct{}

func (graphReferenceSearch) Search(context.Context, string, []string) (knowledge.Result, error) {
	return knowledge.Result{
		Mode: "weknora-hybrid", GraphStatus: "applied", RerankStatus: "applied", Note: "图谱仅为线索，引用须核对原文。",
		Graph: json.RawMessage(`[{"nodes":[{"attributes":["` + strings.Repeat("graph-context", 1000) + `"]}],"edges":[]}]`),
		Hits:  []knowledge.Hit{{ID: "a", Text: "引用原文", Source: "原典", Hash: "hash-a", Revision: 2}},
	}, nil
}

func TestCalculatedReportKeepsCitationsAndPrivateGraphWithoutSendingGraphToWriter(t *testing.T) {
	for _, name := range []string{"chat", "purchased_report", "calculated_report"} {
		t.Run(name, func(t *testing.T) {
			in := askagent.Input{}
			if name != "chat" {
				in.Report = &askagent.Report{Calculated: name == "calculated_report"}
			}
			s := &svc.ServiceContext{Knowledge: &knowledge.Store{Remote: graphReferenceSearch{}}, KnowledgeEnabled: true}
			var observed knowledge.Result
			bindReferences(s, &in, "owner", func(tool, raw string) {
				if tool != "search_knowledge" {
					t.Fatal("unexpected reference tool")
				}
				if err := json.Unmarshal([]byte(raw), &observed); err != nil {
					t.Fatal(err)
				}
			})
			raw, err := in.References(context.Background(), "search_knowledge", "first")
			if err != nil {
				t.Fatal(err)
			}
			var visible knowledge.Result
			if err := json.Unmarshal([]byte(raw), &visible); err != nil {
				t.Fatal(err)
			}
			if len(observed.Graph) == 0 || len(observed.Hits) != 1 || observed.Hits[0].Citation != 1 {
				t.Fatal("private graph or numbered source evidence lost")
			}
			if name == "calculated_report" {
				if len(visible.Graph) != 0 || len(raw) >= len(observed.Graph)/10 {
					t.Fatal("raw graph still consumes report writing context")
				}
				observed.Graph = nil
			}
			if !reflect.DeepEqual(visible, observed) {
				t.Fatal("source text, provenance, retrieval status or ordinary-chat graph changed")
			}
		})
	}
}
