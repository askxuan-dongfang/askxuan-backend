package logic

import (
	"context"

	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/ai-service/internal/types"
	"github.com/askxuan/common"

	"github.com/zeromicro/go-zero/core/logx"
)

// SessionCreateLogic 创建对话会话逻辑
type SessionCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSessionCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SessionCreateLogic {
	return &SessionCreateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// Create 创建会话（校验技能编码，生成会话单号）
func (l *SessionCreateLogic) Create(req *types.SessionCreateReq) (*types.SessionCreateResp, error) {
	if req.UserId == "" || req.SkillCode == "" {
		return nil, common.ErrParam
	}
	skill, ok := model.FindSkillByCode(req.SkillCode)
	if !ok || skill.Status != model.SkillStatusEnabled {
		return nil, common.ErrParam
	}
	session := model.InsertSession(model.AISession{
		UserId:    req.UserId,
		SkillCode: req.SkillCode,
	})
	if req.Question != "" {
		model.InsertMessage(model.AIMessage{
			SessionId: session.Id,
			Role:      model.RoleUser,
			Content:   req.Question,
		})
		model.InsertMessage(model.AIMessage{
			SessionId: session.Id,
			Role:      model.RoleAssistant,
			Content:   buildAssistantPlaceholder(skill.Name),
		})
	}
	return &types.SessionCreateResp{
		Id:        session.Id,
		SessionNo: session.SessionNo,
		SkillCode: session.SkillCode,
		Status:    session.Status,
	}, nil
}

// SessionListLogic 我的对话历史逻辑
type SessionListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSessionListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SessionListLogic {
	return &SessionListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// List 我的会话列表（按 userId 筛选 + 分页）
func (l *SessionListLogic) List(req *types.SessionListReq) (*types.SessionListResp, error) {
	if req.UserId == "" {
		return nil, common.ErrParam
	}
	list, total := model.ListSessions(req.UserId, req.Status, req.Page, req.Size)
	resp := &types.SessionListResp{Total: total, Page: normalizePage(req.Page), Size: normalizeSize(req.Size)}
	resp.List = make([]types.AISession, 0, len(list))
	for _, session := range list {
		resp.List = append(resp.List, toTypesSession(session))
	}
	return resp, nil
}

// SessionDetailLogic 会话详情逻辑
type SessionDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSessionDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SessionDetailLogic {
	return &SessionDetailLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// Detail 会话详情（含消息列表）
func (l *SessionDetailLogic) Detail(req *types.SessionDetailReq) (*types.SessionDetailResp, error) {
	session, ok := model.FindSessionByID(req.Id)
	if !ok {
		return nil, common.ErrSessionNotFound
	}
	messages := model.ListMessagesBySession(req.Id)
	resp := &types.SessionDetailResp{Session: toTypesSession(session), Messages: make([]types.AIMessage, 0, len(messages))}
	for _, message := range messages {
		resp.Messages = append(resp.Messages, toTypesMessage(message))
	}
	return resp, nil
}

// SessionDeleteLogic 关闭对话逻辑
type SessionDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSessionDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SessionDeleteLogic {
	return &SessionDeleteLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// Delete 关闭会话（status=active → closed）
func (l *SessionDeleteLogic) Delete(req *types.SessionDeleteReq) (*types.IdResp, error) {
	if req.Id == 0 {
		return nil, common.ErrParam
	}
	if _, ok := model.CloseSession(req.Id); !ok {
		return nil, common.ErrSessionNotFound
	}
	return &types.IdResp{Id: req.Id}, nil
}

func toTypesSession(session model.AISession) types.AISession {
	return types.AISession{
		Id:        session.Id,
		SessionNo: session.SessionNo,
		UserId:    session.UserId,
		SkillCode: session.SkillCode,
		Status:    session.Status,
		CreatedAt: session.CreatedAt,
		UpdatedAt: session.UpdatedAt,
	}
}

func toTypesMessage(message model.AIMessage) types.AIMessage {
	return types.AIMessage{
		Id:        message.Id,
		SessionId: message.SessionId,
		Role:      message.Role,
		Content:   message.Content,
		Tokens:    message.Tokens,
		CreatedAt: message.CreatedAt,
	}
}

func buildAssistantPlaceholder(skillName string) string {
	return "已收到你的" + skillName + "问题。当前为原型联调回复，请继续补充出生时间、地点或具体事项，正式 AI 推理服务接入后会返回完整解读。"
}

func normalizePage(page int) int {
	if page < 1 {
		return 1
	}
	return page
}

func normalizeSize(size int) int {
	if size < 1 || size > 100 {
		return 20
	}
	return size
}
