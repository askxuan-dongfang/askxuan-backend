package logic

import (
	"context"
	"errors"
	"time"

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
	pendingId, err := l.svcCtx.ConversationModel.CreateTurn(l.ctx, req.Id, req.UserId, req.Content)
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
		input = append(input, provider.Message{Role: m.Role, Content: m.Content})
	}
	resp, err := svcCtx.Provider.Complete(ctx, provider.Request{SystemPrompt: skill.PromptTemplate, Messages: input})
	if err != nil {
		return err
	}
	return svcCtx.ConversationModel.CompleteMessage(ctx, messageId, resp.Content, resp.Tokens)
}
