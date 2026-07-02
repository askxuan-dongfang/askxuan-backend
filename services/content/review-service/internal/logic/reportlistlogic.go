package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/review-service/internal/svc"
	"github.com/askxuan/review-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// ReportListLogic 举报列表逻辑
type ReportListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReportListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReportListLogic {
	return &ReportListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ReportList 平台查看举报列表，支持按 status 筛选 + 分页
func (l *ReportListLogic) ReportList(req *types.ReportListReq) (*types.ReportListResp, error) {
	// TODO: 调用 model.ListReports 查询
	return nil, common.ErrNotImplemented
}
