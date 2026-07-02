package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/review-service/internal/svc"
	"github.com/askxuan/review-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// ReportHandleLogic 处理举报逻辑
type ReportHandleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReportHandleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReportHandleLogic {
	return &ReportHandleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ReportHandle 平台处理举报（handled→隐藏评价 / rejected→驳回）
func (l *ReportHandleLogic) ReportHandle(req *types.ReportHandleReq) (*types.ReportHandleResp, error) {
	// TODO: 校验状态流转 CanTransitReport + 更新举报 + 同步评价状态
	return nil, common.ErrNotImplemented
}
