// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package messageadmin

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/message-service/internal/model"
	"github.com/askxuan/message-service/internal/svc"
	"github.com/askxuan/message-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminPushLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminPushLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminPushLogic {
	return &AdminPushLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminPushLogic) AdminPush(req *types.PushReq) (resp *types.PushResp, err error) {
	id, err := l.svcCtx.PushLogModel.Insert(l.ctx, &model.PushLog{
		UserId:   req.UserId,
		PushType: req.PushType,
		Title:    req.Title,
		Content:  req.Content,
		Status:   "pending",
		BizType:  req.BizType,
		BizId:    req.BizId,
	})
	if err != nil {
		l.Errorf("创建推送日志失败: %v", err)
		return nil, common.ErrSystem
	}

	return &types.PushResp{PushLogId: id, Status: "success"}, nil
}
