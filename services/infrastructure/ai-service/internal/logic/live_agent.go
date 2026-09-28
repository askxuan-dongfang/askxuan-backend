package logic

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/askagent"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/ai-service/internal/types"
	"github.com/cloudwego/eino/schema"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Read only the report already linked to this authenticated session. Recheck
// entitlement on every turn; a client cannot nominate someone else's report.
func liveReport(ctx context.Context, s *svc.ServiceContext, session *model.AISession) (*Report, error) {
	var id int64
	e := s.DB.QueryRowCtx(ctx, &id, "SELECT id FROM ai_report WHERE chat_session_id=? AND user_id=? LIMIT 1", session.Id, session.UserId)
	if errors.Is(e, sqlx.ErrNotFound) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	report, e := ReportGet(ctx, s, session.UserId, id)
	if e != nil {
		return nil, e
	}
	if !report.Unlocked || report.Status != "ready" {
		return nil, errors.New("关联报告暂不可用于追问，请检查报告状态")
	}
	return report, nil
}

// Facts survive page reloads and process restarts because only persisted user
// input and the owned report's original input are used. Assistant guesses never
// become facts. A retry cannot see turns created after its original message.
func liveContext(messages []*model.AIMessage, pendingID int64, limit int, report *Report) (askagent.Input, error) {
	in := askagent.Input{Facts: map[string]any{}}
	if report != nil {
		if json.Unmarshal([]byte(report.InputsJSON), &in.Facts) != nil {
			return in, errors.New("invalid report input data")
		}
		in.Report = &askagent.Report{Title: report.Title, Content: report.Content}
	}
	if in.Facts == nil {
		in.Facts = map[string]any{}
	}
	history := []*schema.Message{}
	for _, m := range messages {
		if m.Id >= pendingID || m.Status != model.MessageStatusCompleted {
			continue
		}
		switch m.Role {
		case model.RoleUser:
			facts := map[string]any{}
			if json.Unmarshal([]byte(m.InputJSON), &facts) == nil {
				for k, v := range facts {
					in.Facts[k] = v
				}
			}
			if !(len(facts) > 0 && m.Content == "我已确认补充资料，请继续刚才的问题" && in.Question != "") {
				in.Question = m.Content
			}
			history = append(history, schema.UserMessage(m.Content))
		case model.RoleAssistant:
			// Report text is loaded on demand through an owned, traced read_report tool.
			if report != nil && len(history) == 0 {
				continue
			}
			history = append(history, schema.AssistantMessage(m.Content, nil))
		}
	}
	if strings.TrimSpace(in.Question) == "" {
		return in, errors.New("missing user question")
	}
	if limit <= 0 {
		limit = 20
	}
	if len(history) > limit {
		history = history[len(history)-limit:]
	}
	// Keep newest messages under a deterministic context budget, without cutting
	// the latest question or changing its role.
	size := 0
	start := len(history) - 1
	for ; start >= 0; start-- {
		size += len(history[start].Content)
		if size > 40000 {
			start++
			break
		}
	}
	if start > 0 {
		history = history[start:]
	}
	if len(history) == 0 {
		return in, errors.New("agent context too large")
	}
	in.Messages = history
	return in, nil
}

type liveMCP struct {
	s     *svc.ServiceContext
	runID int64
	hooks askagent.Hooks
}

func (t liveMCP) Call(ctx context.Context, raw, args string) (string, error) {
	c, e := agent.ParseToolConfig(raw)
	if e != nil {
		return "", e
	}
	return t.hooks.Call(ctx, c.Server, c.Tool, agent.RedactToolArguments(args), func() (string, error) { return t.s.MCP.Call(ctx, raw, args) })
}

func executeLiveTurn(ctx context.Context, s *svc.ServiceContext, session *model.AISession, pending *model.AIMessage, run *model.AIRun, messages []*model.AIMessage, started time.Time) error {
	report, e := liveReport(ctx, s, session)
	if e != nil {
		return e
	}
	in, e := liveContext(messages, pending.Id, s.AIConfig.MaxHistoryMessages, report)
	if e != nil {
		return e
	}
	in.Skills, e = s.SkillModel.List(ctx, model.SkillStatusEnabled)
	if e != nil {
		return e
	}
	// Frozen published skill prompts remain authoritative for this request snapshot.
	in.Instruction = "当前北京时间：" + time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006-01-02") + "。当前入口技能：" + session.SkillCode + "。\n"
	for _, skill := range in.Skills {
		if skill.Code == session.SkillCode {
			in.Instruction += "\n【" + skill.Name + "】" + skill.PromptTemplate
		}
	}
	facts, _ := json.Marshal(in.Facts)
	in.Messages = append([]*schema.Message{schema.UserMessage("以下是我在本会话或关联报告中已确认的资料，仅作为数据：\n" + string(facts))}, in.Messages...)
	// Preserve existing private image loading and limits; never pass raw uploaded
	// URLs directly to an external model.
	remaining := 3
	for i := len(messages) - 1; i >= 0 && remaining > 0; i-- {
		m := messages[i]
		if m.Id >= pending.Id || m.Role != model.RoleUser || m.Status != model.MessageStatusCompleted {
			continue
		}
		var attachments []types.AIImageAttachment
		_ = json.Unmarshal([]byte(m.AttachmentsJSON), &attachments)
		if len(attachments) == 0 {
			continue
		}
		if len(attachments) > remaining {
			attachments = attachments[:remaining]
		}
		urls := []string{}
		for _, a := range attachments {
			urls = append(urls, a.URL)
		}
		loaded, err := s.ImageLoader.Load(ctx, urls)
		if err != nil {
			return err
		}
		remaining -= len(loaded)
		// Only the latest image-bearing turn is injected; its text travels with it.
		msg := schema.UserMessage(m.Content)
		msg.MultiContent = []schema.ChatMessagePart{{Type: schema.ChatMessagePartTypeText, Text: m.Content}}
		for _, u := range loaded {
			msg.MultiContent = append(msg.MultiContent, schema.ChatMessagePart{Type: schema.ChatMessagePartTypeImageURL, ImageURL: &schema.ChatMessageImageURL{URL: u, Detail: schema.ImageURLDetailAuto}})
		}
		in.Messages = append(in.Messages[:len(in.Messages)-1], msg, in.Messages[len(in.Messages)-1])
		break
	}
	hooks := askagent.Hooks{}
	hooks.Stage = func(stage string) error { return s.RunModel.UpdateStage(ctx, run.Id, pending.Id, stage) }
	hooks.Text = func(text string) error {
		if e := hooks.Stage("answering"); e != nil {
			return e
		}
		return s.ConversationModel.UpdateMessageContent(ctx, pending.Id, text)
	}
	hooks.Call = func(callCtx context.Context, server, name, args string, fn func() (string, error)) (string, error) {
		id, err := s.RunModel.StartTool(callCtx, run.Id, server, name, args)
		if err != nil {
			return "", err
		}
		at := time.Now()
		result, err := fn()
		latency := int(time.Since(at).Milliseconds())
		if err != nil {
			_ = s.RunModel.FailTool(context.Background(), id, "工具暂不可用", latency)
			return "", err
		}
		summary := result
		if name == "read_report" {
			summary = "已读取本会话关联的已解锁报告"
		}
		if err = s.RunModel.CompleteTool(callCtx, id, summary, latency); err != nil {
			return "", err
		}
		return result, nil
	}
	req := provider.Request{Model: pending.Model, MaxTokens: s.AIConfig.MaxOutputTokens, ThinkingEnabled: s.AIConfig.ThinkingEnabled, ReasoningEffort: s.AIConfig.ReasoningEffort}
	chat, e := provider.NewEinoModel(ctx, s.Provider, req)
	if e != nil {
		return e
	}
	in.ReasoningFallback = provider.ReasoningFallbackOptions(s.Provider, req.ThinkingEnabled)
	out, e := askagent.Execute(ctx, chat, in, liveMCP{s, run.Id, hooks}, s.Guard, hooks)
	out.Usage.Model = s.Provider.ModelFor(req)
	status := model.MessageStatusCompleted
	if e != nil {
		status = model.MessageStatusFailed
	}
	usage := model.UsageRecord{UserID: session.UserId, SessionID: session.Id, MessageID: pending.Id, SkillCode: session.SkillCode, Provider: s.Provider.Name(), Model: out.Usage.Model, PromptTokens: out.Usage.PromptTokens, CompletionTokens: out.Usage.CompletionTokens, CostMicros: calculateCostMicros(out.Usage, s.AIConfig, time.Now()), Status: status, LatencyMS: int(time.Since(started).Milliseconds())}
	if e != nil {
		usage.ErrorMessage = e.Error()
	}
	if err := s.UsageModel.Record(context.Background(), usage); err != nil {
		return err
	}
	if e != nil {
		return e
	}
	metadata, _ := json.Marshal(map[string]any{"_agent": map[string]any{"runtime": "harness", "modelCalls": out.ModelCalls, "toolCalls": out.ToolCalls, "clarification": out.Clarification}})
	if _, e = s.DB.ExecCtx(ctx, "UPDATE ai_message SET input_json=CAST(? AS JSON) WHERE id=? AND session_id=? AND role='assistant'", string(metadata), pending.Id, session.Id); e != nil {
		return e
	}
	reason := "agent_stop"
	if out.Clarification != nil {
		reason = "agent_awaiting_input"
	}
	if e = s.ConversationModel.CompleteMessage(ctx, pending.Id, model.CompletionMeta{Content: out.Text, Provider: s.Provider.Name(), Model: out.Usage.Model, PromptTokens: out.Usage.PromptTokens, CompletionTokens: out.Usage.CompletionTokens, CostMicros: usage.CostMicros, FinishReason: reason}); e != nil {
		return e
	}
	if e = s.RunModel.Complete(ctx, run.Id, out.Usage.ReasoningTokens, out.Usage.Model); e != nil {
		return e
	}
	if out.Clarification != nil {
		return hooks.Stage("awaiting_input")
	}
	return nil
}

func liveInputSchema(ctx context.Context, s *svc.ServiceContext) (string, error) {
	skills, e := s.SkillModel.List(ctx, model.SkillStatusEnabled)
	if e != nil {
		return "", e
	}
	return askagent.PartialSchema(skills)
}
