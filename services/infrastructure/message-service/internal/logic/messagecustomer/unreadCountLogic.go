// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package messagecustomer

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/message-service/internal/svc"
	"github.com/askxuan/message-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UnreadCountLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUnreadCountLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnreadCountLogic {
	return &UnreadCountLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UnreadCountLogic) UnreadCount(req *types.UnreadCountReq) (resp *types.UnreadCountResp, err error) {
	count, err := l.svcCtx.MessageModel.UnreadCount(l.ctx, req.UserId)
	if err != nil {
		l.Errorf("查询未读消息数失败: %v", err)
		return nil, common.ErrSystem
	}

	return &types.UnreadCountResp{Count: count}, nil
}
