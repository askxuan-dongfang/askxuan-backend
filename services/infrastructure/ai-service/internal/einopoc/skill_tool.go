package einopoc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

type MCPCaller interface {
	Call(context.Context, string, string) (string, error)
}

// SkillTool binds a server-reviewed skill and user-supplied facts. Model-supplied
// arguments cannot invent missing birth data, change the endpoint, or select an
// arbitrary remote tool. This probe supports deterministic, read-only tools only.
type SkillTool struct {
	skill    model.AISkill
	inputs   string
	question string
	client   MCPCaller
	guard    *agent.Guard
	now      time.Time
}

type ToolState struct {
	Inputs string    `json:"inputs"`
	At     time.Time `json:"at"`
}

// Only call between runner executions; the tool is task-local and sequential.
func (t *SkillTool) State() ToolState { return ToolState{Inputs: t.inputs, At: t.now} }
func (t *SkillTool) Restore(s ToolState) error {
	if len(s.Inputs) > 8000 || !json.Valid([]byte(s.Inputs)) || s.At.IsZero() {
		return errors.New("invalid tool state")
	}
	t.inputs = s.Inputs
	t.now = s.At
	return nil
}

func NewSkillTool(skill model.AISkill, inputs map[string]any, question string, client MCPCaller, guard *agent.Guard) (*SkillTool, error) {
	if client == nil || guard == nil {
		return nil, errors.New("missing skill dependencies")
	}
	if skill.Status != model.SkillStatusEnabled {
		return nil, errors.New("skill is disabled")
	}
	switch skill.Code {
	case "bazi", "ziwei", "qimen", "tarot", "liuyao":
	default:
		return nil, errors.New("skill is outside the read-only tool allowlist")
	}
	c, err := agent.ParseToolConfig(skill.ToolConfig)
	if err != nil || !c.Enabled || c.Tool == "" {
		return nil, errors.New("skill MCP tool is not enabled")
	}
	var parsed agent.InputSchema
	if json.Unmarshal([]byte(skill.InputSchema), &parsed) != nil || len(parsed.Fields) == 0 {
		return nil, errors.New("skill requires a reviewed input schema")
	}
	if _, err = guard.Validate(`{"fields":[]}`, question, nil); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(inputs)
	if err != nil {
		return nil, err
	}
	return &SkillTool{skill: skill, inputs: string(raw), question: question, client: client, guard: guard, now: time.Now()}, nil
}

func (t *SkillTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "calculate_" + t.skill.Code, Desc: t.skill.Name + "：" + t.skill.Description + "。使用用户已确认的结构化资料进行计算；缺少或无效资料时暂停并请求用户补充。不得猜测资料。", ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, nil
}

func (t *SkillTool) InvokableRun(ctx context.Context, args string, _ ...tool.Option) (string, error) {
	var modelArgs map[string]any
	if json.Unmarshal([]byte(args), &modelArgs) != nil || modelArgs == nil || len(modelArgs) != 0 {
		return "", errors.New("skill arguments must be supplied by the user, not the model")
	}
	raw := t.inputs
	if targeted, hasData, answer := tool.GetResumeContext[string](ctx); targeted && hasData {
		raw = answer
	}
	var inputs map[string]any
	if len(raw) > 8000 || json.Unmarshal([]byte(raw), &inputs) != nil || inputs == nil {
		return "", tool.Interrupt(ctx, "请以 JSON 对象补充有效资料；不要猜测未知信息。")
	}
	validated, err := t.guard.Validate(t.skill.InputSchema, t.question, inputs)
	if err != nil {
		var schema agent.InputSchema
		_ = json.Unmarshal([]byte(t.skill.InputSchema), &schema)
		missing := []string{}
		for _, field := range schema.Fields {
			value, exists := inputs[field.Key]
			if field.Required && (!exists || value == nil || value == "") {
				label := field.Label
				if label == "" {
					label = field.Key
				}
				missing = append(missing, label)
			}
		}
		if len(missing) > 0 {
			return "", tool.Interrupt(ctx, "请补充："+strings.Join(missing, "、"))
		}
		return "", tool.Interrupt(ctx, "请检查资料的格式与可选值，修正后继续。")
	}
	arguments, err := agent.BuildToolArguments(t.skill.Code, t.question, validated, t.now)
	if err != nil || arguments == "" {
		return "", tool.Interrupt(ctx, "计算所需资料不足或无效，请补充后再继续。")
	}
	// This tool is task-local and executed sequentially. Retain user-confirmed
	// corrections so a retry after resume does not ask for the same facts again.
	t.inputs = raw
	result, err := t.client.Call(ctx, t.skill.ToolConfig, arguments)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		// Do not forward upstream response bodies, addresses or credentials to the model.
		return `{"ok":false,"error":"calculation_unavailable","message":"尚无计算依据，可在预算内重试；不能编造结果"}`, nil
	}
	if strings.TrimSpace(result) == "" || len(result) > 16384 {
		return `{"ok":false,"error":"invalid_tool_result"}`, nil
	}
	encoded, err := json.Marshal(map[string]any{"ok": true, "evidence": result})
	return string(encoded), err
}
