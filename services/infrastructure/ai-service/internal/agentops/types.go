// Package agentops manages reviewed configurations separately from live requests.
package agentops

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/model"
)

var (
	ErrConflict    = errors.New("配置已变更，请重新加载后操作")
	ErrUntested    = errors.New("请先运行当前草稿的评测集，全部通过后再发布")
	ErrUnavailable = errors.New("智能体管理暂不可用，请确认数据库迁移已完成")
	ErrExpired     = errors.New("调试已过期或服务已重启，请重新开始")
)

type RequestError struct{ Message string }

func (e *RequestError) Error() string { return e.Message }
func invalid(message string) error    { return &RequestError{Message: message} }

type SkillPolicy struct {
	Code    string `json:"code"`
	Enabled bool   `json:"enabled"`
	Prompt  string `json:"prompt"`
	UseTool bool   `json:"useTool"`
}
type Config struct {
	Name            string           `json:"name"`
	Instruction     string           `json:"instruction"`
	Model           string           `json:"model"`
	MaxOutputTokens int              `json:"maxOutputTokens"`
	Skills          []SkillPolicy    `json:"skills"`
	Evaluation      []EvaluationCase `json:"evaluation"`
}

// Frozen includes the reviewed input/tool contracts, not just editable prompts.
type Frozen struct {
	Config   Config              `json:"config"`
	Skills   []model.AISkill     `json:"skills"`
	Approval *EvaluationApproval `json:"approval,omitempty"`
}
type State struct {
	Revision      int64  `db:"revision" json:"revision"`
	Draft         string `db:"draft_json" json:"-"`
	ActiveVersion int64  `db:"active_version" json:"activeVersion"`
}
type Version struct {
	ID         int64  `db:"id" json:"id"`
	Definition string `db:"definition_json" json:"-"`
	Actor      string `db:"actor" json:"actor"`
	Note       string `db:"note" json:"note"`
	CreatedAt  string `db:"create_time" json:"createdAt"`
}
type Audit struct {
	ID        int64  `db:"id" json:"id"`
	Action    string `db:"action" json:"action"`
	VersionID int64  `db:"version_id" json:"versionId"`
	Actor     string `db:"actor" json:"actor"`
	Note      string `db:"note" json:"note"`
	CreatedAt string `db:"create_time" json:"createdAt"`
}
type SkillInfo struct {
	Code          string          `json:"code"`
	Name          string          `json:"name"`
	Version       string          `json:"version"`
	Description   string          `json:"description"`
	InputSchema   json.RawMessage `json:"inputSchema"`
	ToolName      string          `json:"toolName"`
	ToolAvailable bool            `json:"toolAvailable"`
	EinoSupported bool            `json:"einoSupported"`
}
type Workspace struct {
	RuntimeMode string `json:"runtimeMode"`
	State
	DraftSaved         bool        `json:"draftSaved"`
	Draft              Config      `json:"draft"`
	Catalog            []SkillInfo `json:"catalog"`
	Active             *Config     `json:"active"`
	Versions           []Version   `json:"versions"`
	Audit              []Audit     `json:"audit"`
	Tested             bool        `json:"tested"`
	LiveEnabled        bool        `json:"liveEnabled"`
	PersistentRecovery bool        `json:"persistentRecovery"`
	Rollout            Rollout     `json:"rollout"`
}
type Event struct {
	Stage string `json:"stage"`
	Label string `json:"label"`
	At    string `json:"at"`
}
type DebugRun struct {
	ID               string  `json:"id"`
	Actor            string  `json:"actor"`
	Revision         int64   `json:"revision"`
	ProviderRevision int64   `json:"providerRevision"`
	Kind             string  `json:"kind"`
	SkillCode        string  `json:"skillCode"`
	Model            string  `json:"model"`
	Status           string  `json:"status"`
	Result           string  `json:"result"`
	Clarification    string  `json:"clarification"`
	InterruptID      string  `json:"interruptId"`
	Error            string  `json:"error"`
	Events           []Event `json:"events"`
	ModelAttempts    int     `json:"modelAttempts"`
	ToolAttempts     int     `json:"toolAttempts"`
	PromptTokens     *int    `json:"promptTokens"`
	CompletionTokens *int    `json:"completionTokens"`
	StartedAt        string  `json:"startedAt"`
}
type Repository interface {
	State(context.Context) (State, error)
	Save(context.Context, int64, string, string) error
	Publish(context.Context, int64, int64, string, string) (int64, error)
	Rollback(context.Context, int64, int64, string, string) error
	Version(context.Context, int64) (Version, error)
	Versions(context.Context) ([]Version, error)
	Audit(context.Context) ([]Audit, error)
	Tested(context.Context, int64, int64) (bool, error)
	PutDebug(context.Context, DebugRun) error
	Debug(context.Context, string) (DebugRun, error)
	DebugList(context.Context) ([]DebugRun, error)
}

func Default(skills []*model.AISkill) Config {
	c := Config{Name: "问事助手", Instruction: "根据用户已确认的资料和工具依据提供清晰、审慎的文化与生活参考。缺少资料时先补问。", MaxOutputTokens: 512, Skills: []SkillPolicy{}}
	for _, s := range skills {
		tc, _ := agent.ParseToolConfig(s.ToolConfig)
		c.Skills = append(c.Skills, SkillPolicy{Code: s.Code, Enabled: s.Status == model.SkillStatusEnabled, Prompt: s.PromptTemplate, UseTool: tc.Enabled})
	}
	return c
}
func Freeze(c Config, catalog []*model.AISkill) (Frozen, error) {
	c.Name = strings.TrimSpace(c.Name)
	c.Instruction = strings.TrimSpace(c.Instruction)
	c.Model = strings.TrimSpace(c.Model)
	if c.Name == "" || len([]rune(c.Name)) > 40 || c.Instruction == "" || len(c.Instruction) > 8000 || len(c.Model) > 100 || c.MaxOutputTokens < 64 || c.MaxOutputTokens > 4096 || len(c.Skills) == 0 || len(c.Skills) > 100 {
		return Frozen{}, invalid("请检查名称、职责、输出上限与技能配置")
	}
	byCode := map[string]*model.AISkill{}
	for _, s := range catalog {
		byCode[s.Code] = s
	}
	f := Frozen{Config: c, Skills: []model.AISkill{}}
	if err := validateCases(c.Evaluation, c.Skills, false); err != nil {
		return Frozen{}, err
	}
	seen := map[string]bool{}
	enabled := 0
	for _, p := range c.Skills {
		base, ok := byCode[p.Code]
		if !ok || seen[p.Code] || len(p.Prompt) > 16000 {
			return Frozen{}, invalid("技能不存在、重复或提示词过长")
		}
		seen[p.Code] = true
		s := *base
		s.PromptTemplate = p.Prompt
		s.Status = "disabled"
		if p.Enabled {
			if base.Status != model.SkillStatusEnabled {
				return Frozen{}, invalid("源技能已停用，不能发布启用")
			}
			s.Status = model.SkillStatusEnabled
			enabled++
		}
		if p.Code == model.SkillCodeGeneral && !p.Enabled {
			return Frozen{}, invalid("日常问事是自动路由的兜底技能，请保持启用")
		}
		tc, e := agent.ParseToolConfig(base.ToolConfig)
		if e != nil {
			return Frozen{}, invalid("技能工具契约无效")
		}
		if p.UseTool && (!tc.Enabled || tc.Tool == "") {
			return Frozen{}, invalid("技能未配置可用计算工具")
		}
		if !p.UseTool {
			s.ToolConfig = `{"enabled":false}`
		}
		if !json.Valid([]byte(s.InputSchema)) {
			return Frozen{}, invalid("技能输入规则无效")
		}
		f.Skills = append(f.Skills, s)
	}
	if enabled == 0 {
		return Frozen{}, invalid("至少启用一个技能")
	}
	if _, exists := byCode[model.SkillCodeGeneral]; exists && !seen[model.SkillCodeGeneral] {
		return Frozen{}, invalid("请保留日常问事兜底技能")
	}
	return f, nil
}
func Decode(raw string) (Frozen, error) {
	var f Frozen
	err := json.Unmarshal([]byte(raw), &f)
	if err != nil || f.Config.Name == "" || len(f.Skills) == 0 {
		return Frozen{}, ErrUnavailable
	}
	return f, nil
}
func (f Frozen) Skill(code string) (model.AISkill, error) {
	for _, s := range f.Skills {
		if s.Code == code && s.Status == model.SkillStatusEnabled {
			return s, nil
		}
	}
	return model.AISkill{}, invalid("技能未启用")
}

// SkillView is a request-local copy, so a later publish cannot mutate a running task.
type SkillView struct {
	Frozen  Frozen
	Version int64
}

func (v *SkillView) List(_ context.Context, status string) ([]*model.AISkill, error) {
	out := []*model.AISkill{}
	for _, s := range v.Frozen.Skills {
		if status == "" || status == s.Status {
			x := s
			x.Version = fmt.Sprintf("%.20s@a%d", s.Version, v.Version)
			out = append(out, &x)
		}
	}
	return out, nil
}
func (v *SkillView) FindByCode(ctx context.Context, code string) (*model.AISkill, error) {
	all, _ := v.List(ctx, "")
	for _, s := range all {
		if s.Code == code {
			return s, nil
		}
	}
	return nil, invalid("技能未纳入当前发布版本")
}
