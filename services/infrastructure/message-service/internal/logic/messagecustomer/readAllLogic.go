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

type ReadAllLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReadAllLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReadAllLogic {
	return &ReadAllLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ReadAllLogic) ReadAll(req *types.ReadAllReq) (resp *types.ReadAllResp, err error) {
	count, err := l.svcCtx.MessageModel.MarkAllRead(l.ctx, req.UserId)
	if err != nil {
		l.Errorf("全部标记已读失败: %v", err)
		return nil, common.ErrSystem
	}

	return &types.ReadAllResp{Count: count}, nil
}
