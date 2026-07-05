package logic

import (
	"context"
	"errors"

	"github.com/askxuan/diy-service/internal/svc"
	"github.com/askxuan/diy-service/rpc/diy"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetBlessingTaskByTaskNoLogic 按任务编号查询加持任务（供 master/temple MQ 消费者使用）
type GetBlessingTaskByTaskNoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetBlessingTaskByTaskNoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBlessingTaskByTaskNoLogic {
	return &GetBlessingTaskByTaskNoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetBlessingTaskByTaskNoLogic) GetBlessingTaskByTaskNo(req *diy.GetBlessingTaskByTaskNoReq) (*diy.BlessingTask, error) {
	task, err := l.svcCtx.BlessingTaskModel.FindByTaskNo(l.ctx, req.TaskNo)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "加持任务不存在")
		}
		return nil, err
	}
	return modelTaskToRPC(task), nil
}
