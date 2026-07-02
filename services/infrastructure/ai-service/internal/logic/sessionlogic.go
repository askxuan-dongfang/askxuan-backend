package logic

import (
	"context"

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
	return nil, common.ErrNotImplemented
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
	return nil, common.ErrNotImplemented
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
	return nil, common.ErrNotImplemented
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
	return nil, common.ErrNotImplemented
}
