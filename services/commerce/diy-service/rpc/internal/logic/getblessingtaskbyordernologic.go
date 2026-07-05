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

// GetBlessingTaskByOrderNoLogic 按 DIY 订单号查询加持任务
type GetBlessingTaskByOrderNoLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetBlessingTaskByOrderNoLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBlessingTaskByOrderNoLogic {
	return &GetBlessingTaskByOrderNoLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetBlessingTaskByOrderNoLogic) GetBlessingTaskByOrderNo(req *diy.GetBlessingTaskByOrderNoReq) (*diy.BlessingTask, error) {
	task, err := l.svcCtx.BlessingTaskModel.FindByDiyOrderNo(l.ctx, req.DiyOrderNo)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "加持任务不存在")
		}
		return nil, err
	}
	return modelTaskToRPC(task), nil
}
