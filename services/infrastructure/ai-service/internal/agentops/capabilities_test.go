package agentops

import (
	"context"
	"encoding/json"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/settings"
	"testing"
)

func TestCapabilityMembershipAndToolPermissionsAreFrozen(t *testing.T) {
	skills := catalog().items
	skills = append(skills, &model.AISkill{Code: "bazi", Name: "八字", Status: "enabled", InputSchema: `{"fields":[{"key":"birthDate","type":"date","required":true}]}`, PromptTemplate: "reviewed", ToolConfig: `{"enabled":true,"server":"taibu","tool":"bazi"}`})
	original := Default(skills)
	published, err := Freeze(original, skills)
	if err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"remove", "disable", "tool-off"} {
		t.Run(action, func(t *testing.T) {
			draft := Default(skills)
			switch action {
			case "remove":
				draft.Skills = draft.Skills[:1]
			case "disable":
				draft.Skills[1].Enabled = false
			case "tool-off":
				draft.Skills[1].UseTool = false
			}
			frozen, err := Freeze(draft, skills)
			if err != nil {
				t.Fatal(err)
			}
			view := &SkillView{Frozen: frozen, Version: 2}
			enabled, err := view.List(context.Background(), model.SkillStatusEnabled)
			if err != nil {
				t.Fatal(err)
			}
			if action != "tool-off" && len(enabled) != 1 {
				t.Fatal("removed/disabled skill remained available", enabled)
			}
			if action == "remove" {
				if _, err = view.FindByCode(context.Background(), "bazi"); err == nil {
					t.Fatal("removed skill still bound")
				}
			}
			if action == "tool-off" {
				var cfg struct{ Enabled bool }
				_ = json.Unmarshal([]byte(frozen.Skills[1].ToolConfig), &cfg)
				if cfg.Enabled {
					t.Fatal("disabled tool still authorized")
				}
			}
			if published.Skills[1].Status != "enabled" || published.Skills[1].ToolConfig != skills[1].ToolConfig {
				t.Fatal("draft mutated source or published snapshot")
			}
		})
	}
	skills[1].Status = "disabled"
	if _, err = Freeze(original, skills); err == nil {
		t.Fatal("stopped source enabled by draft")
	}
}
func TestCapabilityRemovalRequiresEvaluationCleanup(t *testing.T) {
	skills := catalog().items
	skills = append(skills, &model.AISkill{Code: "dream", Status: "enabled", InputSchema: `{"fields":[]}`, ToolConfig: `{"enabled":false}`})
	draft := Default(skills)
	draft.Evaluation = []EvaluationCase{{ID: "dream", Name: "梦境", SkillCode: "dream", Question: "测试", Inputs: map[string]any{}, MinChars: 20}}
	draft.Skills = draft.Skills[:1]
	if _, err := Freeze(draft, skills); err == nil {
		t.Fatal("dangling evaluation accepted")
	}
	draft.Evaluation = nil
	if _, err := Freeze(draft, skills); err != nil {
		t.Fatal(err)
	}
}

// The workspace returns source metadata without mutating the chosen membership.
type capabilityRepo struct {
	Repository
	draft string
}

func (r capabilityRepo) State(context.Context) (State, error) {
	return State{Revision: 1, Draft: r.draft}, nil
}
func (r capabilityRepo) Versions(context.Context) ([]Version, error)        { return nil, nil }
func (r capabilityRepo) Audit(context.Context) ([]Audit, error)             { return nil, nil }
func (r capabilityRepo) Tested(context.Context, int64, int64) (bool, error) { return false, nil }
func TestWorkspaceExposesReviewedDefaultsWithoutReaddingRemovedCapabilities(t *testing.T) {
	skills := catalog()
	base := Default(skills.items)
	frozen, err := Freeze(base, skills.items)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(frozen)
	skills.items = append(skills.items, &model.AISkill{Code: "naming", Name: "姓名", Status: "disabled", Version: "3.1", PromptTemplate: "reviewed defaults", SourceRef: "reviewed/version", InputSchema: `{"fields":[]}`, ToolConfig: `{"enabled":true,"server":"builtin","tool":"naming"}`})
	m := New(capabilityRepo{draft: string(raw)}, skills, func() (*settings.Snapshot, int64) { return nil, 1 }, nil, false)
	w, err := m.Workspace(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(w.Draft.Skills) != 1 || len(w.Catalog) != 2 {
		t.Fatal("catalog refresh changed draft membership")
	}
	s := w.Catalog[1]
	if s.DefaultPrompt != "reviewed defaults" || s.SourceStatus != "disabled" || s.ToolServer != "builtin" || s.SourceRef != "reviewed/version" {
		t.Fatalf("missing reviewed metadata: %+v", s)
	}
}
