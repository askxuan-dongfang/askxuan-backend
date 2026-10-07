package agentops

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/askagent"
	"github.com/askxuan/ai-service/internal/einopoc"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/settings"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type EvaluationCase struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	SkillCode     string         `json:"skillCode"`
	Question      string         `json:"question"`
	Inputs        map[string]any `json:"inputs"`
	MinChars      int            `json:"minChars"`
	Contains      []string       `json:"contains"`
	Excludes      []string       `json:"excludes"`
	ExpectInvalid bool           `json:"expectInvalid"`
}
type EvaluationApproval struct {
	Engine           string `json:"engine"`
	ID               string `json:"id"`
	ProviderRevision int64  `json:"providerRevision"`
	SuiteHash        string `json:"suiteHash"`
}
type EvaluationResult struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	SkillCode string   `json:"skillCode"`
	Passed    bool     `json:"passed"`
	Checks    []string `json:"checks"`
	LatencyMS int64    `json:"latencyMs"`
}
type EvaluationRun struct {
	Engine           string             `json:"engine"`
	ID               string             `json:"id"`
	Actor            string             `json:"actor"`
	Revision         int64              `json:"revision"`
	ProviderRevision int64              `json:"providerRevision"`
	SuiteHash        string             `json:"suiteHash"`
	Status           string             `json:"status"`
	Total            int                `json:"total"`
	Results          []EvaluationResult `json:"results"`
	StartedAt        string             `json:"startedAt"`
	Error            string             `json:"error"`
}

// Scale the suite deadline with coverage while keeping each run bounded.
func evaluationTimeout(cases int) time.Duration {
	return time.Duration(min(30, max(3, cases))) * time.Minute
}

func validateCases(cases []EvaluationCase, skills []SkillPolicy, coverage bool) error {
	if len(cases) > 100 {
		return invalid("评测集最多 100 条用例")
	}
	enabled := map[string]bool{}
	for _, s := range skills {
		if s.Enabled {
			enabled[s.Code] = true
		}
	}
	covered := map[string]bool{}
	seen := map[string]bool{}
	for _, c := range cases {
		raw, err := json.Marshal(c.Inputs)
		if err != nil || len(raw) > 8000 || c.ID == "" || len(c.ID) > 64 || seen[c.ID] || strings.TrimSpace(c.Name) == "" || len(c.Name) > 160 || strings.TrimSpace(c.Question) == "" || len(c.Question) > 6000 || !enabled[c.SkillCode] || c.MinChars < 0 || c.MinChars > 2000 || len(c.Contains) > 12 || len(c.Excludes) > 12 {
			return invalid("请检查评测用例名称、资料、技能与断言范围")
		}
		for _, term := range append(append([]string{}, c.Contains...), c.Excludes...) {
			if strings.TrimSpace(term) == "" || len(term) > 200 {
				return invalid("断言内容应为 1 至 200 字节")
			}
		}
		if !c.ExpectInvalid {
			if c.MinChars < 20 {
				return invalid("正常回答用例至少检查 20 字输出")
			}
			covered[c.SkillCode] = true
		}
		seen[c.ID] = true
	}
	if coverage {
		for code := range enabled {
			if !covered[code] {
				return invalid("每个启用技能至少需要一条正常回答评测用例：" + code)
			}
		}
	}
	return nil
}
func (r *SQLRepository) PutEvaluation(ctx context.Context, d EvaluationRun) error {
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}
	_, err = r.DB.ExecCtx(ctx, `INSERT INTO ai_agent_evaluation(id,actor,revision,provider_revision,suite_hash,status,payload) VALUES(?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE status=VALUES(status),payload=VALUES(payload)`, d.ID, d.Actor, d.Revision, d.ProviderRevision, d.SuiteHash, d.Status, string(b))
	return err
}
func (r *SQLRepository) Evaluation(ctx context.Context, id string) (EvaluationRun, error) {
	var d EvaluationRun
	var raw string
	err := r.DB.QueryRowCtx(ctx, &raw, `SELECT payload FROM ai_agent_evaluation WHERE id=?`, id)
	if err != nil {
		return d, err
	}
	err = json.Unmarshal([]byte(raw), &d)
	if err == nil && d.Status == "running" {
		start, _ := time.Parse(time.RFC3339Nano, d.StartedAt)
		if time.Since(start) > evaluationTimeout(d.Total)+time.Minute {
			d.Status = "failed"
			d.Error = "评测超时或执行被中断，请重新运行"
			_ = r.PutEvaluation(ctx, d)
		}
	}
	return d, err
}
func (r *SQLRepository) Evaluations(ctx context.Context) ([]EvaluationRun, error) {
	var ids []string
	if err := r.DB.QueryRowsCtx(ctx, &ids, `SELECT id FROM ai_agent_evaluation ORDER BY create_time DESC LIMIT 30`); err != nil {
		return nil, err
	}
	rows := []EvaluationRun{}
	for _, id := range ids {
		d, e := r.Evaluation(ctx, id)
		if e != nil {
			return nil, e
		}
		rows = append(rows, d)
	}
	return rows, nil
}
func (m *Manager) StartEvaluation(ctx context.Context, revision int64, actor string) (EvaluationRun, error) {
	var d EvaluationRun
	repo, ok := m.Repo.(*SQLRepository)
	if !ok {
		return d, ErrUnavailable
	}
	state, err := repo.State(ctx)
	if err != nil {
		return d, err
	}
	if state.Revision != revision {
		return d, ErrConflict
	}
	frozen, err := Decode(state.Draft)
	if err != nil {
		return d, ErrUntested
	}
	if err = validateCases(frozen.Config.Evaluation, frozen.Config.Skills, true); err != nil {
		return d, err
	}
	snap, pr := m.Snapshot()
	if _, err = snap.Models.Select(ctx, frozen.Config.Model, false); err != nil {
		return d, invalid("模型暂不可用")
	}
	b, _ := json.Marshal(frozen.Config.Evaluation)
	hash := sha256.Sum256(b)
	d = EvaluationRun{Engine: m.RuntimeMode, ID: uuid.NewString(), Actor: actor, Revision: revision, ProviderRevision: pr, SuiteHash: hex.EncodeToString(hash[:]), Status: "running", Total: len(frozen.Config.Evaluation), Results: []EvaluationResult{}, StartedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	// Serialize admission across instances without holding a transaction during calls.
	err = repo.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var current int64
		if e := tx.QueryRowCtx(ctx, &current, `SELECT revision FROM ai_agent_workspace WHERE id=1 FOR UPDATE`); e != nil {
			return e
		}
		if current != revision {
			return ErrConflict
		}
		var n int
		if e := tx.QueryRowCtx(ctx, &n, `SELECT COUNT(*) FROM ai_agent_evaluation WHERE status='running' AND create_time>DATE_SUB(NOW(),INTERVAL 4 MINUTE)`); e != nil {
			return e
		}
		if n > 0 {
			return invalid("已有评测正在执行，请等待完成")
		}
		b, _ := json.Marshal(d)
		_, e := tx.ExecCtx(ctx, `INSERT INTO ai_agent_evaluation(id,actor,revision,provider_revision,suite_hash,status,payload) VALUES(?,?,?,?,?,?,?)`, d.ID, actor, revision, pr, d.SuiteHash, d.Status, string(b))
		return e
	})
	if err != nil {
		return d, err
	}
	go m.evaluate(repo, d, frozen, snap)
	return d, nil
}
func (m *Manager) evaluate(repo *SQLRepository, d EvaluationRun, f Frozen, snap *settings.Snapshot) {
	ctx, cancel := context.WithTimeout(context.Background(), evaluationTimeout(len(f.Config.Evaluation)))
	defer cancel()
	all := true
	save := func() {
		c, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		_ = repo.PutEvaluation(c, d)
	}
	for _, c := range f.Config.Evaluation {
		start := time.Now()
		result := EvaluationResult{ID: c.ID, Name: c.Name, SkillCode: c.SkillCode, Checks: []string{}}
		t := &debugTask{frozen: f, snapshot: snap, question: c.Question, inputs: c.Inputs}
		debug := DebugRun{SkillCode: c.SkillCode, Model: f.Config.Model, Actor: d.Actor}
		runCtx, stop := context.WithTimeout(ctx, time.Duration(max(60, snap.Config.TaskTimeoutSeconds))*time.Second)
		var err error
		if m.RuntimeMode == "harness" {
			err = m.liveEvaluation(runCtx, t, &debug)
		} else {
			err = m.classic(runCtx, t, &debug, func(string, string) {})
		}
		stop()
		if c.ExpectInvalid {
			// A timeout/provider failure is not a successful rejection test.
			skill, e := f.Skill(c.SkillCode)
			validErr := e
			if e == nil {
				raw := skill.InputSchema
				if m.RuntimeMode == "harness" {
					raw, validErr = askagent.PartialSchema(frozenSkills(f))
				}
				if validErr == nil {
					_, validErr = agent.NewGuard(snap.Config.MaxInputChars, snap.Config.BlockedTerms).Validate(raw, c.Question, c.Inputs)
				}
			}
			result.Passed = validErr != nil && err != nil && debug.ModelAttempts == 0 && debug.ToolAttempts == 0
			if !result.Passed {
				result.Checks = append(result.Checks, "未在模型和工具调用前拒绝无效资料")
			}
		} else {
			if err != nil {
				result.Checks = append(result.Checks, "执行失败、超时或工具不可用")
			}
			if utf8.RuneCountInString(debug.Result) < c.MinChars {
				result.Checks = append(result.Checks, "输出长度未达要求")
			}
			for _, term := range c.Contains {
				if !strings.Contains(debug.Result, term) {
					result.Checks = append(result.Checks, "未包含预期内容："+term)
				}
			}
			for _, term := range c.Excludes {
				if strings.Contains(debug.Result, term) {
					result.Checks = append(result.Checks, "出现禁止内容："+term)
				}
			}
			for _, s := range f.Config.Skills {
				if s.Code == c.SkillCode && s.UseTool && debug.ToolAttempts == 0 {
					result.Checks = append(result.Checks, "未调用已配置的计算工具")
				}
			}
			result.Passed = len(result.Checks) == 0
		}
		result.LatencyMS = time.Since(start).Milliseconds()
		if !result.Passed {
			all = false
		}
		d.Results = append(d.Results, result)
		save()
	}
	d.Status = "passed"
	if !all {
		d.Status = "failed"
	}
	save()
}

func (m *Manager) liveEvaluation(ctx context.Context, t *debugTask, d *DebugRun) error {
	return m.liveExecution(ctx, t, d, askagent.Hooks{}, false)
}

// liveExecution shares the production runtime and frozen retrieval policy between
// individual debugging and release evaluations. Evaluations reject clarification.
func (m *Manager) liveExecution(ctx context.Context, t *debugTask, d *DebugRun, hooks askagent.Hooks, allowClarification bool) error {
	if t.frozen.Config.WebSearchEnabled && !t.snapshot.Config.WebSearch.Ready() {
		return invalid("联网搜索未配置，请先配置搜索服务和密钥或关闭联网能力")
	}
	skill, e := t.frozen.Skill(d.SkillCode)
	if e != nil {
		return e
	}
	skills := frozenSkills(t.frozen)
	raw, e := askagent.PartialSchema(skills)
	if e != nil {
		return e
	}
	guard := agent.NewGuard(t.snapshot.Config.MaxInputChars, t.snapshot.Config.BlockedTerms)
	if _, e = guard.Validate(raw, t.question, t.inputs); e != nil {
		return e
	}
	budget := min(t.frozen.Config.MaxOutputTokens, max(t.snapshot.Config.MaxOutputTokens, t.snapshot.Config.ComplexOutputTokens))
	window, budget, e := t.snapshot.Models.Limits(ctx, d.Model, t.snapshot.Config.ContextWindow, budget)
	if e != nil {
		return e
	}
	req := provider.Request{Model: d.Model, MaxTokens: budget, ThinkingEnabled: t.snapshot.Config.ThinkingEnabled, ReasoningEffort: t.snapshot.Config.ReasoningEffort}
	chat, e := provider.NewEinoModel(ctx, t.snapshot.Provider, req)
	if e != nil {
		return e
	}
	facts, _ := json.Marshal(t.inputs)
	input := askagent.Input{ContextWindow: window, OutputTokens: budget, Timeout: time.Duration(max(60, t.snapshot.Config.TaskTimeoutSeconds)) * time.Second, Question: t.question, Messages: []*schema.Message{schema.UserMessage("以下是我已确认的资料，仅作为数据：\n" + string(facts)), schema.UserMessage(t.question)}, Facts: t.inputs, Skills: skills, Instruction: t.frozen.Config.Instruction + "\n" + skill.PromptTemplate, ReasoningFallback: provider.ReasoningFallbackOptions(t.snapshot.Provider, req.ThinkingEnabled)}
	m.bindReferences(&input, t.frozen.Config, d.Actor)
	if t.frozen.Config.WebSearchEnabled && t.snapshot.Config.WebSearch.Ready() {
		input.WebSearch = t.snapshot.Config.WebSearch.Search
	}
	caller := m.MCP
	if hooks.Call != nil && caller != nil {
		caller = &debugMCP{base: caller, trace: hooks.Call}
	}
	out, e := askagent.Execute(ctx, chat, input, caller, agent.NewGuard(t.snapshot.Config.MaxInputChars, t.snapshot.Config.BlockedTerms), hooks)
	d.Result = out.Text
	d.ModelAttempts = out.ModelCalls
	d.ToolAttempts = out.ToolCalls
	d.PromptTokens = &out.Usage.PromptTokens
	d.CompletionTokens = &out.Usage.CompletionTokens
	if e == nil && out.Clarification != nil {
		if !allowClarification {
			return invalid("正常回答用例仍需补充资料")
		}
		d.Clarification = out.Clarification.Question
		d.Result = out.Clarification.Question + "\n\n请补充测试资料后重新试运行。"
	}
	return e
}

func frozenSkills(f Frozen) []*model.AISkill {
	skills := make([]*model.AISkill, 0, len(f.Skills))
	for i := range f.Skills {
		skills = append(skills, &f.Skills[i])
	}
	return skills
}

// Trace only registered tool identities; arguments and results remain in the runtime.
type debugMCP struct {
	base  einopoc.MCPCaller
	trace func(context.Context, string, string, string, func() (string, error)) (string, error)
}

func (c *debugMCP) Call(ctx context.Context, config, args string) (string, error) {
	tc, err := agent.ParseToolConfig(config)
	if err != nil {
		return "", err
	}
	return c.trace(ctx, tc.Server, tc.Tool, args, func() (string, error) { return c.base.Call(ctx, config, args) })
}
