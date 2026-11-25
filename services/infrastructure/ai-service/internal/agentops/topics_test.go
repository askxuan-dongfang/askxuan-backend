package agentops

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/model"
)

func TestTopicPolicyFreezesFormAndRestrictsCapabilities(t *testing.T) {
	skills := catalog().items
	for _, tool := range agent.ReviewedTools() {
		if tool.Code == "dream" || tool.Code == "qimen" {
			raw, _ := json.Marshal(tool.InputSchema)
			skills = append(skills, &model.AISkill{Code: tool.Code, Name: tool.Name, Status: "enabled", InputSchema: string(raw), ToolConfig: fmt.Sprintf(`{"enabled":true,"tool":%q}`, tool.Code)})
		}
	}
	const base = "87654321-4321-4321-4321-123456789087"
	config := func() Config {
		draft := Default(skills)
		draft.KnowledgeEnabled = true
		draft.KnowledgeBaseIDs = []string{base}
		draft.Topics = []TopicPolicy{{
			Code: "dream", KnowledgeBaseIDs: []string{base}, SkillCodes: []string{"general"}, ToolCodes: []string{"dream", "qimen"},
			RequiredFields: []string{"eventTime"}, AdditionalFields: []TopicField{{Key: "topic_context", Label: "  近期关注  ", Required: true}},
			Chapters: []string{" 依据 ", "分析", "行动", ""},
		}}
		return draft
	}
	draft := config()
	frozen, err := Freeze(draft, skills)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(frozen)
	version, err := Decode(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	p := version.Config.Topic("dream")
	if p == nil || !slices.Equal(p.Chapters, []string{"依据", "分析", "行动"}) || p.AdditionalFields[0].Label != "近期关注" || version.Config.Topic("qimen") != nil {
		t.Fatalf("wrong published policy: %+v", p)
	}
	if draft.Topics[0].Chapters[0] != " 依据 " {
		t.Fatal("freezing mutated the draft")
	}
	byCode := map[string]*model.AISkill{}
	for _, skill := range skills {
		byCode[skill.Code] = skill
	}
	form, err := p.Form(byCode)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"dream", "eventTime", "topic_context"} {
		if !slices.ContainsFunc(form.Fields, func(field agent.Field) bool { return field.Key == key && field.Required }) {
			t.Errorf("lost required field %s", key)
		}
	}
	for name, change := range map[string]func(*Config){
		"out-of-scope knowledge": func(c *Config) { c.KnowledgeBaseIDs = nil },
		"disabled knowledge":     func(c *Config) { c.KnowledgeEnabled = false },
		"disabled global tool":   func(c *Config) { c.Skills[1].UseTool = false; c.Skills[2].UseTool = false },
		"unknown skill":          func(c *Config) { c.Topics[0].SkillCodes = []string{"missing"} },
		"missing core tool":      func(c *Config) { c.Topics[0].ToolCodes = []string{"qimen"} },
		"unknown required data":  func(c *Config) { c.Topics[0].RequiredFields = []string{"invalid"} },
		"shadowed field":         func(c *Config) { c.Topics[0].AdditionalFields[0].Key = "dream" },
		"duplicate chapter":      func(c *Config) { c.Topics[0].Chapters[1] = "依据" },
	} {
		t.Run(name, func(t *testing.T) {
			draft := config()
			change(&draft)
			if _, err := Freeze(draft, skills); err == nil {
				t.Fatal("accepted invalid topic policy")
			}
		})
	}
}
