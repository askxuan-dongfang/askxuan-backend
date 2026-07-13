package logic

import (
	"context"

	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/mq"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/ai-service/internal/types"
	"github.com/askxuan/common"

	"github.com/zeromicro/go-zero/core/logx"
)

// MessageSendLogic 发送问事消息逻辑
type MessageSendLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMessageSendLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MessageSendLogic {
	return &MessageSendLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// Send 发送问事消息
// 1. 落库 user 消息 2. 发送 ai.divination MQ 事件异步处理 3. 返回 accepted
func (l *MessageSendLogic) Send(req *types.MessageSendReq) (*types.MessageSendResp, error) {
	if req.Id == 0 || req.UserId == "" || req.Content == "" {
		return nil, common.ErrParam
	}
	session, ok := model.FindSessionByID(req.Id)
	if !ok {
		return nil, common.ErrSessionNotFound
	}
	if session.UserId != req.UserId || session.Status != model.SessionStatusActive {
		return nil, common.ErrParam
	}
	model.InsertMessage(model.AIMessage{
		SessionId: req.Id,
		Role:      model.RoleUser,
		Content:   req.Content,
	})
	_ = l.svcCtx.MqProducer.Publish(l.ctx, mq.AIDivination{
		SessionId: req.Id,
		UserId:    req.UserId,
		SkillCode: session.SkillCode,
		Content:   req.Content,
	})
	model.InsertMessage(model.AIMessage{
		SessionId: req.Id,
		Role:      model.RoleAssistant,
		Content:   "问题已受理。当前原型环境会先保存对话，正式推理服务接入后将异步补全解读结果。",
	})
	return &types.MessageSendResp{SessionId: req.Id, Status: "accepted"}, nil
}

// MessageListLogic 查询会话消息列表逻辑
type MessageListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMessageListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MessageListLogic {
	return &MessageListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *MessageListLogic) List(req *types.MessageListReq) (*types.MessageListResp, error) {
	if req.Id == 0 {
		return nil, common.ErrParam
	}
	if _, ok := model.FindSessionByID(req.Id); !ok {
		return nil, common.ErrSessionNotFound
	}
	all := model.ListMessagesBySession(req.Id)
	page := normalizePage(req.Page)
	size := normalizeSize(req.Size)
	start := (page - 1) * size
	if start > len(all) {
		start = len(all)
	}
	end := start + size
	if end > len(all) {
		end = len(all)
	}
	resp := &types.MessageListResp{
		Total: int64(len(all)),
		List:  make([]types.AIMessage, 0, end-start),
		Page:  page,
		Size:  size,
	}
	for _, message := range all[start:end] {
		resp.List = append(resp.List, toTypesMessage(message))
	}
	return resp, nil
}
