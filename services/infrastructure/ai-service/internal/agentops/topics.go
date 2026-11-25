package agentops

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/model"
)

type TopicField struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Required bool   `json:"required"`
}

type TopicPolicy struct {
	Code             string       `json:"code"`
	KnowledgeBaseIDs []string     `json:"knowledgeBaseIds"`
	SkillCodes       []string     `json:"skillCodes"`
	ToolCodes        []string     `json:"toolCodes"`
	RequiredFields   []string     `json:"requiredFields"`
	AdditionalFields []TopicField `json:"additionalFields"`
	Chapters         []string     `json:"chapters"`
}

type ReportTopic struct {
	Code              string   `json:"code" db:"code"`
	Title             string   `json:"title" db:"title"`
	ChaptersJSON      string   `json:"-" db:"chapters_json"`
	Chapters          []string `json:"chapters" db:"-"`
	RequiredToolCodes []string `json:"requiredToolCodes" db:"-"`
}

func (r *SQLRepository) reportTopics(ctx context.Context) ([]ReportTopic, error) {
	topics := []ReportTopic{}
	err := r.DB.QueryRowsPartialCtx(ctx, &topics, `SELECT p.code,p.title,p.chapters_json FROM ai_report_product p JOIN ai_skill s ON s.code=p.code WHERE p.enabled=1 ORDER BY s.sort_order,p.code`)
	if err != nil {
		return nil, err
	}
	for i := range topics {
		if err := json.Unmarshal([]byte(topics[i].ChaptersJSON), &topics[i].Chapters); err != nil {
			return nil, err
		}
		topics[i].RequiredToolCodes = append([]string{}, agent.RequiredReportTools(topics[i].Code)...)
	}
	return topics, nil
}

func (c *Config) Topic(code string) *TopicPolicy {
	if c != nil {
		for i := range c.Topics {
			if c.Topics[i].Code == code {
				return &c.Topics[i]
			}
		}
	}
	return nil
}

var topicFieldKey = regexp.MustCompile(`^topic_[a-z0-9_]{1,40}$`)

// Form keeps the main skill's field contracts and adds fields needed by optional tools.
func (p TopicPolicy) Form(skills map[string]*model.AISkill) (agent.InputSchema, error) {
	form := agent.InputSchema{Fields: []agent.Field{}}
	known := map[string]bool{}
	requiredTools := agent.RequiredReportTools(p.Code)
	for i, code := range append([]string{p.Code}, p.ToolCodes...) {
		primary := i == 0
		if !primary && slices.Contains(requiredTools, code) {
			continue
		}
		skill := skills[code]
		if skill == nil {
			return form, invalid("表单依赖的技能不可用")
		}
		var schema agent.InputSchema
		if err := json.Unmarshal([]byte(agent.GuidedInputSchema(code, skill.InputSchema)), &schema); err != nil {
			return form, invalid("技能资料格式无效")
		}
		for _, field := range schema.Fields {
			if known[field.Key] {
				continue
			}
			known[field.Key] = true
			if !primary {
				field.Required, field.RequiredWhen, field.DefaultValue = false, nil, ""
			}
			field.Required = field.Required || slices.Contains(p.RequiredFields, field.Key)
			form.Fields = append(form.Fields, field)
		}
	}
	for _, key := range p.RequiredFields {
		if !known[key] {
			return form, invalid("必填资料必须来自专题表单")
		}
	}
	for _, field := range p.AdditionalFields {
		if !topicFieldKey.MatchString(field.Key) || known[field.Key] || field.Label == "" || utf8.RuneCountInString(field.Label) > 50 || strings.ContainsAny(field.Label, "\r\n") {
			return form, invalid("自定义资料的名称或标识无效")
		}
		known[field.Key] = true
		form.Fields = append(form.Fields, agent.Field{Key: field.Key, Label: field.Label, Type: "text", Required: field.Required})
	}
	return form, nil
}

func (f *Frozen) freezeTopics() error {
	if len(f.Config.Topics) == 0 {
		return nil
	}
	enabled := map[string]*model.AISkill{}
	for i := range f.Skills {
		skill := &f.Skills[i]
		if skill.Status == model.SkillStatusEnabled {
			enabled[skill.Code] = skill
		}
	}
	f.Config.Topics = slices.Clone(f.Config.Topics)
	seen := map[string]bool{}
	for i := range f.Config.Topics {
		p := &f.Config.Topics[i]
		if enabled[p.Code] == nil || seen[p.Code] {
			return invalid("专题技能未启用或专题配置重复")
		}
		seen[p.Code] = true
		bad := func(message string) error {
			return invalid(fmt.Sprintf("%s：%s", enabled[p.Code].Name, message))
		}
		for _, id := range p.KnowledgeBaseIDs {
			if !f.Config.KnowledgeEnabled || !slices.Contains(f.Config.KnowledgeBaseIDs, id) {
				return bad("请选择助手已启用的知识库")
			}
		}
		for _, code := range p.SkillCodes {
			if enabled[code] == nil || code == p.Code {
				return bad("附加技能须在助手中启用")
			}
		}
		for _, code := range p.ToolCodes {
			skill := enabled[code]
			if skill == nil || !agent.IsReadOnlyTool(code) {
				return bad("请选择助手已启用的计算工具")
			}
			tool, err := agent.ParseToolConfig(skill.ToolConfig)
			if err != nil || !tool.Enabled || tool.Tool != code {
				return bad("请选择助手已启用的计算工具")
			}
		}
		for _, code := range agent.RequiredReportTools(p.Code) {
			if !slices.Contains(p.ToolCodes, code) {
				return bad("请保留专题所需的基础计算工具")
			}
		}
		if len(p.AdditionalFields) > 12 {
			return bad("自定义资料最多 12 项")
		}
		chapters := make([]string, 0, len(p.Chapters))
		for _, title := range p.Chapters {
			title = strings.TrimSpace(title)
			if title == "" {
				continue
			}
			if strings.ContainsAny(title, "#\r\n") || utf8.RuneCountInString(title) > 60 || slices.Contains(chapters, title) {
				return bad("报告章节不可重复，每项须为 1 至 60 字的标题")
			}
			chapters = append(chapters, title)
		}
		if len(chapters) < 3 || len(chapters) > 12 {
			return bad("报告须有 3 至 12 个章节")
		}
		p.Chapters = chapters
		p.KnowledgeBaseIDs = append([]string{}, p.KnowledgeBaseIDs...)
		p.SkillCodes = append([]string{}, p.SkillCodes...)
		p.ToolCodes = append([]string{}, p.ToolCodes...)
		p.RequiredFields = append([]string{}, p.RequiredFields...)
		p.AdditionalFields = append([]TopicField{}, p.AdditionalFields...)
		for i := range p.AdditionalFields {
			p.AdditionalFields[i].Label = strings.TrimSpace(p.AdditionalFields[i].Label)
		}
		if _, err := p.Form(enabled); err != nil {
			return bad(err.Error())
		}
	}
	return nil
}
