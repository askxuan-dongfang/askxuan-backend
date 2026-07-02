// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package messagecustomer

import (
	"context"
	"strconv"

	"github.com/askxuan/common"
	"github.com/askxuan/message-service/internal/svc"
	"github.com/askxuan/message-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteMessageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteMessageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteMessageLogic {
	return &DeleteMessageLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteMessageLogic) DeleteMessage(req *types.DeleteMessageReq) (resp *types.IdResp, err error) {
	id, err := strconv.ParseInt(req.Id, 10, 64)
	if err != nil {
		l.Errorf("解析消息ID失败: %v", err)
		return nil, common.ErrSystem
	}

	if err := l.svcCtx.MessageModel.DeleteMessage(l.ctx, id); err != nil {
		l.Errorf("删除消息失败: %v", err)
		return nil, common.ErrSystem
	}

	return &types.IdResp{Id: id}, nil
}
