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

// CompleteBlessingTaskLogic 完成加持任务（供 master-service 法师完成加持使用）
// 设置状态为 completed + 证书 URL + complete_time
type CompleteBlessingTaskLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCompleteBlessingTaskLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CompleteBlessingTaskLogic {
	return &CompleteBlessingTaskLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CompleteBlessingTaskLogic) CompleteBlessingTask(req *diy.CompleteBlessingTaskReq) (*diy.BlessingTask, error) {
	// certificate_urls 为 JSON 数组字符串，直接透传给 model
	if err := l.svcCtx.BlessingTaskModel.UpdateComplete(l.ctx, req.Id, req.CertificateUrls); err != nil {
		return nil, err
	}
	// 返回更新后的任务
	task, err := l.svcCtx.BlessingTaskModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "加持任务不存在")
		}
		return nil, err
	}
	return modelTaskToRPC(task), nil
}
