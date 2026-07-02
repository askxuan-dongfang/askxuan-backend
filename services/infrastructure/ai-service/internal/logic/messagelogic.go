package logic

import (
	"context"

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
	return nil, common.ErrNotImplemented
}
