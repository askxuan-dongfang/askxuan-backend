package logic

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/ai-service/internal/types"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type SessionCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSessionCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SessionCreateLogic {
	return &SessionCreateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}
func (l *SessionCreateLogic) Create(req *types.SessionCreateReq) (*types.SessionCreateResp, error) {
	if req.UserId == "" {
		return nil, common.ErrParam
	}
	selectionMode := "explicit"
	if req.SkillCode == "" || req.SkillCode == "auto" {
		skills, listErr := l.svcCtx.SkillModel.List(l.ctx, model.SkillStatusEnabled)
		if listErr != nil {
			return nil, common.ErrSystem
		}
		req.SkillCode = agent.RouteSkill(req.Question, req.Inputs, skills)
		selectionMode = "auto"
	}
	skill, err := l.svcCtx.SkillModel.FindByCode(l.ctx, req.SkillCode)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrParamInvalid
		}
		return nil, common.ErrSystem
	}
	if skill.Status != model.SkillStatusEnabled {
		return nil, common.ErrParamInvalid
	}
	schemaJSON := skill.InputSchema
	if selectionMode == "auto" && len(req.Inputs) == 0 {
		// Auto-routed conversations may begin before a complete profile exists.
		// The reviewed skill prompt asks for the missing fields before any tool runs.
		schemaJSON = `{"fields":[]}`
	}
	inputJSON, err := l.svcCtx.Guard.Validate(schemaJSON, req.Question, req.Inputs)
	if err != nil {
		if errors.Is(err, agent.ErrUnsafeContent) || errors.Is(err, agent.ErrInvalidInputs) || errors.Is(err, agent.ErrInputTooLong) {
			return nil, common.ErrParamInvalid
		}
		return nil, common.ErrSystem
	}
	if strings.TrimSpace(req.Question) == "" && len(req.Inputs) > 0 {
		req.Question = "请根据所填资料进行分析"
	}
	attachmentsJSON, err := encodeAttachments(req.Attachments)
	if err != nil {
		return nil, common.ErrParamInvalid
	}
	if strings.TrimSpace(req.Question) == "" && len(req.Attachments) > 0 {
		req.Question = "请分析我上传的图片"
	}
	if strings.TrimSpace(req.Question) != "" {
		if err := l.svcCtx.UsageModel.Acquire(l.ctx, req.UserId, l.svcCtx.AIConfig.MinuteRequestLimit, l.svcCtx.AIConfig.DailyRequestLimit); err != nil {
			if errors.Is(err, model.ErrQuotaExceeded) {
				return nil, common.ErrTooManyRequest
			}
			return nil, common.ErrSystem
		}
	}
	session, pendingId, err := l.svcCtx.ConversationModel.CreateSession(l.ctx, req.UserId, req.SkillCode, selectionMode, skill.Version, req.Question, inputJSON, attachmentsJSON)
	if err != nil {
		l.Errorf("创建AI会话失败: %v", err)
		return nil, common.ErrSystem
	}
	if pendingId > 0 {
		processAsync(l.svcCtx, session.Id, pendingId)
	}
	return &types.SessionCreateResp{Id: session.Id, SessionNo: session.SessionNo, SkillCode: session.SkillCode, Status: session.Status, MessageId: pendingId}, nil
}

type SessionListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSessionListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SessionListLogic {
	return &SessionListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}
func (l *SessionListLogic) List(req *types.SessionListReq) (*types.SessionListResp, error) {
	if req.UserId == "" {
		return nil, common.ErrParam
	}
	page, size := normalizePage(req.Page), normalizeSize(req.Size)
	list, total, err := l.svcCtx.ConversationModel.ListSessions(l.ctx, req.UserId, req.Status, page, size)
	if err != nil {
		return nil, common.ErrSystem
	}
	resp := &types.SessionListResp{Total: total, Page: page, Size: size, List: make([]types.AISession, 0, len(list))}
	for _, s := range list {
		resp.List = append(resp.List, toTypesSession(*s))
	}
	return resp, nil
}

type SessionDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSessionDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SessionDetailLogic {
	return &SessionDetailLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}
func (l *SessionDetailLogic) Detail(req *types.SessionDetailReq) (*types.SessionDetailResp, error) {
	if req.UserId == "" {
		return nil, common.ErrForbidden
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
	messages, _, err := l.svcCtx.ConversationModel.ListMessages(l.ctx, req.Id, 1, 100)
	if err != nil {
		return nil, common.ErrSystem
	}
	resp := &types.SessionDetailResp{Session: toTypesSession(*s), Messages: make([]types.AIMessage, 0, len(messages))}
	for _, m := range messages {
		resp.Messages = append(resp.Messages, toTypesMessage(*m))
	}
	return resp, nil
}

type SessionDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSessionDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SessionDeleteLogic {
	return &SessionDeleteLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}
func (l *SessionDeleteLogic) Delete(req *types.SessionDeleteReq) (*types.IdResp, error) {
	if req.Id == 0 || req.UserId == "" {
		return nil, common.ErrParam
	}
	ok, err := l.svcCtx.ConversationModel.CloseSession(l.ctx, req.Id, req.UserId)
	if err != nil {
		return nil, common.ErrSystem
	}
	if !ok {
		return nil, common.ErrSessionNotFound
	}
	return &types.IdResp{Id: req.Id}, nil
}

func toTypesSession(s model.AISession) types.AISession {
	return types.AISession{Id: s.Id, SessionNo: s.SessionNo, UserId: s.UserId, SkillCode: s.SkillCode, SelectionMode: s.SelectionMode, SkillVersion: s.SkillVersion, Title: s.Title, Status: s.Status, CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt}
}
func toTypesMessage(m model.AIMessage) types.AIMessage {
	inputs := map[string]interface{}{}
	_ = json.Unmarshal([]byte(m.InputJSON), &inputs)
	attachments := []types.AIImageAttachment{}
	_ = json.Unmarshal([]byte(m.AttachmentsJSON), &attachments)
	return types.AIMessage{
		Id: m.Id, SessionId: m.SessionId, Role: m.Role, Content: m.Content, Inputs: inputs, Attachments: attachments, RunId: m.RunId, Stage: m.Stage,
		Tokens: m.Tokens, PromptTokens: m.PromptTokens, CompletionTokens: m.CompletionTokens,
		Provider: m.Provider, Model: m.Model, CostMicros: m.CostMicros, FinishReason: m.FinishReason,
		Status: m.Status, ErrorMessage: m.ErrorMessage,
		Retryable: m.Role == model.RoleAssistant && m.Status == model.MessageStatusFailed, CreatedAt: m.CreatedAt,
	}
}

func encodeAttachments(attachments []types.AIImageAttachment) (string, error) {
	if len(attachments) > 3 {
		return "", agent.ErrInvalidInputs
	}
	for _, attachment := range attachments {
		if strings.TrimSpace(attachment.URL) == "" || !strings.HasPrefix(strings.ToLower(strings.TrimSpace(attachment.ContentType)), "image/") {
			return "", agent.ErrInvalidInputs
		}
	}
	encoded, err := json.Marshal(attachments)
	return string(encoded), err
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
