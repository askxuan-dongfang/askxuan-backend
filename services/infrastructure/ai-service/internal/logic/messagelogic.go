package logic

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/ai-service/internal/types"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type MessageSendLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMessageSendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MessageSendLogic {
	return &MessageSendLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}
func (l *MessageSendLogic) Send(req *types.MessageSendReq) (*types.MessageSendResp, error) {
	if req.Id == 0 || req.UserId == "" || req.Content == "" {
		return nil, common.ErrParam
	}
	session, err := l.svcCtx.ConversationModel.FindSession(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrSessionNotFound
		}
		return nil, common.ErrSystem
	}
	if session.UserId != req.UserId {
		return nil, common.ErrForbidden
	}
	skill, err := l.svcCtx.SkillModel.FindByCode(l.ctx, session.SkillCode)
	if err != nil {
		return nil, common.ErrSystem
	}
	schemaJSON := skill.InputSchema
	if len(req.Inputs) == 0 {
		// 结构化资料只要求在技能会话首轮提交；后续追问可以只发自然语言。
		schemaJSON = `{"fields":[]}`
	}
	inputJSON, err := l.svcCtx.Guard.Validate(schemaJSON, req.Content, req.Inputs)
	if err != nil {
		if errors.Is(err, agent.ErrUnsafeContent) || errors.Is(err, agent.ErrInvalidInputs) || errors.Is(err, agent.ErrInputTooLong) {
			return nil, common.ErrParamInvalid
		}
		return nil, common.ErrSystem
	}
	if err := l.svcCtx.UsageModel.Acquire(l.ctx, req.UserId, l.svcCtx.AIConfig.MinuteRequestLimit, l.svcCtx.AIConfig.DailyRequestLimit); err != nil {
		if errors.Is(err, model.ErrQuotaExceeded) {
			return nil, common.ErrTooManyRequest
		}
		return nil, common.ErrSystem
	}
	pendingId, err := l.svcCtx.ConversationModel.CreateTurn(l.ctx, req.Id, req.UserId, req.Content, inputJSON)
	if err != nil {
		switch {
		case errors.Is(err, sqlx.ErrNotFound):
			return nil, common.ErrSessionNotFound
		case errors.Is(err, model.ErrConversationForbidden):
			return nil, common.ErrForbidden
		case errors.Is(err, model.ErrSessionClosed):
			return nil, common.ErrParamInvalid
		default:
			l.Errorf("创建AI消息事务失败: %v", err)
			return nil, common.ErrSystem
		}
	}
	processAsync(l.svcCtx, req.Id, pendingId)
	return &types.MessageSendResp{SessionId: req.Id, MessageId: pendingId, Status: model.MessageStatusPending}, nil
}

type MessageListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMessageListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MessageListLogic {
	return &MessageListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}
func (l *MessageListLogic) List(req *types.MessageListReq) (*types.MessageListResp, error) {
	if req.Id == 0 || req.UserId == "" {
		return nil, common.ErrParam
	}
	s, err := l.svcCtx.ConversationModel.FindSession(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrSessionNotFound
		}
		return nil, common.ErrSystem
	}
	if s.UserId != req.UserId {
		return nil, common.ErrForbidden
	}
	page, size := normalizePage(req.Page), normalizeSize(req.Size)
	messages, total, err := l.svcCtx.ConversationModel.ListMessages(l.ctx, req.Id, page, size)
	if err != nil {
		return nil, common.ErrSystem
	}
	resp := &types.MessageListResp{Total: total, List: make([]types.AIMessage, 0, len(messages)), Page: page, Size: size}
	for _, m := range messages {
		resp.List = append(resp.List, toTypesMessage(*m))
	}
	return resp, nil
}

type MessageRetryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMessageRetryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MessageRetryLogic {
	return &MessageRetryLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}
func (l *MessageRetryLogic) Retry(req *types.MessageRetryReq) (*types.MessageSendResp, error) {
	if req.Id == 0 || req.MessageId == 0 || req.UserId == "" {
		return nil, common.ErrParam
	}
	if err := l.svcCtx.UsageModel.Acquire(l.ctx, req.UserId, l.svcCtx.AIConfig.MinuteRequestLimit, l.svcCtx.AIConfig.DailyRequestLimit); err != nil {
		if errors.Is(err, model.ErrQuotaExceeded) {
			return nil, common.ErrTooManyRequest
		}
		return nil, common.ErrSystem
	}
	ok, err := l.svcCtx.ConversationModel.PrepareRetry(l.ctx, req.Id, req.MessageId, req.UserId)
	if err != nil {
		return nil, common.ErrSystem
	}
	if !ok {
		return nil, common.ErrParamInvalid
	}
	processAsync(l.svcCtx, req.Id, req.MessageId)
	return &types.MessageSendResp{SessionId: req.Id, MessageId: req.MessageId, Status: model.MessageStatusPending}, nil
}

func processAsync(svcCtx *svc.ServiceContext, sessionId, messageId int64) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if err := processMessage(ctx, svcCtx, sessionId, messageId); err != nil {
			logx.Errorf("AI Provider处理失败 session=%d message=%d: %v", sessionId, messageId, err)
			_ = svcCtx.ConversationModel.FailMessage(context.Background(), messageId, err.Error())
		}
	}()
}
func processMessage(ctx context.Context, svcCtx *svc.ServiceContext, sessionId, messageId int64) error {
	startedAt := time.Now()
	s, err := svcCtx.ConversationModel.FindSession(ctx, sessionId)
	if err != nil {
		return err
	}
	skill, err := svcCtx.SkillModel.FindByCode(ctx, s.SkillCode)
	if err != nil {
		return err
	}
	messages, err := svcCtx.ConversationModel.ListAllMessages(ctx, sessionId)
	if err != nil {
		return err
	}
	input := make([]provider.Message, 0, len(messages))
	for _, m := range messages {
		if m.Id == messageId || m.Status != model.MessageStatusCompleted {
			continue
		}
		content := m.Content
		if m.Role == model.RoleUser {
			if structured := agent.StructuredContext(skill.InputSchema, m.InputJSON); structured != "" {
				content += "\n\n" + structured
			}
		}
		input = append(input, provider.Message{Role: m.Role, Content: content})
	}
	if max := svcCtx.AIConfig.MaxHistoryMessages; max > 0 && len(input) > max {
		input = input[len(input)-max:]
	}
	systemPrompt := strings.TrimSpace(skill.PromptTemplate) + "\n\n你提供的是文化与生活参考，不替代医疗、法律、金融等专业意见；不得宣称确定预言，不诱导用户恐慌、转账或高风险行为。"
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != model.RoleUser {
			continue
		}
		toolResult, err := svcCtx.MCP.Call(ctx, skill.ToolConfig, messages[i].InputJSON)
		if err != nil {
			return err
		}
		if toolResult != "" {
			systemPrompt += "\n\n以下是受控 MCP 工具的计算结果，只能作为本次回答的结构化依据：\n" + toolResult
		}
		break
	}
	var streamed strings.Builder
	lastPersisted := time.Now()
	resp, err := svcCtx.Provider.Stream(ctx, provider.Request{SystemPrompt: systemPrompt, Messages: input, MaxTokens: svcCtx.AIConfig.MaxOutputTokens}, func(delta string) error {
		streamed.WriteString(delta)
		if err := svcCtx.Guard.ValidateOutput(streamed.String()); err != nil {
			return err
		}
		if time.Since(lastPersisted) < 120*time.Millisecond && streamed.Len() < 256 {
			return nil
		}
		lastPersisted = time.Now()
		return svcCtx.ConversationModel.UpdateMessageContent(ctx, messageId, streamed.String())
	})
	if err != nil {
		_ = svcCtx.UsageModel.Record(context.Background(), model.UsageRecord{
			UserID: s.UserId, SessionID: sessionId, MessageID: messageId, SkillCode: s.SkillCode,
			Provider: svcCtx.Provider.Name(), Model: svcCtx.Provider.Model(), Status: model.MessageStatusFailed,
			LatencyMS: int(time.Since(startedAt).Milliseconds()), ErrorMessage: err.Error(),
		})
		return err
	}
	if err := svcCtx.Guard.ValidateOutput(resp.Content); err != nil {
		return err
	}
	costMicros := int64(math.Round(float64(resp.PromptTokens)*svcCtx.AIConfig.InputCostPerMillion + float64(resp.CompletionTokens)*svcCtx.AIConfig.OutputCostPerMillion))
	meta := model.CompletionMeta{
		Content: resp.Content, PromptTokens: resp.PromptTokens, CompletionTokens: resp.CompletionTokens,
		Provider: svcCtx.Provider.Name(), Model: svcCtx.Provider.Model(), CostMicros: costMicros, FinishReason: resp.FinishReason,
	}
	if err := svcCtx.ConversationModel.CompleteMessage(ctx, messageId, meta); err != nil {
		return err
	}
	return svcCtx.UsageModel.Record(context.Background(), model.UsageRecord{
		UserID: s.UserId, SessionID: sessionId, MessageID: messageId, SkillCode: s.SkillCode,
		Provider: meta.Provider, Model: meta.Model, PromptTokens: meta.PromptTokens, CompletionTokens: meta.CompletionTokens,
		CostMicros: meta.CostMicros, Status: model.MessageStatusCompleted, LatencyMS: int(time.Since(startedAt).Milliseconds()),
	})
}

type UsageSummaryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUsageSummaryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UsageSummaryLogic {
	return &UsageSummaryLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *UsageSummaryLogic) Summary(req *types.UsageSummaryReq) (*types.UsageSummaryResp, error) {
	if req.UserId == "" {
		return nil, common.ErrForbidden
	}
	summary, err := l.svcCtx.UsageModel.Summary(l.ctx, req.UserId)
	if err != nil {
		return nil, common.ErrSystem
	}
	return &types.UsageSummaryResp{
		MinuteRequests: summary.MinuteRequests, MinuteLimit: l.svcCtx.AIConfig.MinuteRequestLimit,
		DailyRequests: summary.DailyRequests, DailyLimit: l.svcCtx.AIConfig.DailyRequestLimit,
		DailyTokens: summary.DailyTokens, DailyCostMicros: summary.DailyCostMicros,
	}, nil
}
