package logic

import (
	"context"

	"github.com/askxuan/audit-service/internal/svc"
	"github.com/askxuan/audit-service/internal/types"
	"github.com/askxuan/common"

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

// ReportHandle 处理举报（handled/rejected）
func (l *ReportHandleLogic) ReportHandle(req *types.ReportHandleReq) (*types.ReportHandleResp, error) {
	// TODO: 校验状态流转 CanTransitReport + 更新状态 + 记录处理结果
	return nil, common.ErrNotImplemented
}
