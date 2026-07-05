package logic

import (
	"context"

	"github.com/askxuan/diy-service/rpc/diy"
	"github.com/askxuan/diy-service/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

// UpdateBlessingTaskStatusLogic 更新加持任务状态（供 master-service 法师接受/开始/拒绝任务使用）
type UpdateBlessingTaskStatusLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateBlessingTaskStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateBlessingTaskStatusLogic {
	return &UpdateBlessingTaskStatusLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateBlessingTaskStatusLogic) UpdateBlessingTaskStatus(req *diy.UpdateBlessingTaskStatusReq) (*diy.BlessingTask, error) {
	task, err := l.svcCtx.BlessingTaskModel.UpdateStatus(l.ctx, req.Id, req.Status)
	if err != nil {
		return nil, err
	}
	return modelTaskToRPC(task), nil
}
