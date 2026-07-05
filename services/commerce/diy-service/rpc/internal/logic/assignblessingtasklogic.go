package logic

import (
	"context"

	"github.com/askxuan/diy-service/rpc/diy"
	"github.com/askxuan/diy-service/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

// AssignBlessingTaskLogic 分配法师（供 temple-service 寺院分配法师使用）
// 校验状态流转 dispatched/rejected → assigned，更新 master_code/status/assign_time
type AssignBlessingTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAssignBlessingTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AssignBlessingTaskLogic {
	return &AssignBlessingTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *AssignBlessingTaskLogic) AssignBlessingTask(req *diy.AssignBlessingTaskReq) (*diy.BlessingTask, error) {
	task, err := l.svcCtx.BlessingTaskModel.Assign(l.ctx, req.Id, req.MasterCode)
	if err != nil {
		return nil, err
	}
	return modelTaskToRPC(task), nil
}
