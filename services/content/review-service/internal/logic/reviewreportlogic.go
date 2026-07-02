package logic

import (
	"context"

	"github.com/askxuan/review-service/internal/model"
	"github.com/askxuan/review-service/internal/svc"
	"github.com/askxuan/review-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// ReviewReportLogic 举报评价逻辑
type ReviewReportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReviewReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReviewReportLogic {
	return &ReviewReportLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ReviewReport 寺院/法师对不当评价发起举报
func (l *ReviewReportLogic) ReviewReport(req *types.ReviewReportReq) (*types.ReviewReportResp, error) {
	report := model.CreateReport(model.ReviewReport{
		ReviewId:   req.Id,
		ReporterId: req.ReporterId,
		Reason:     req.Reason,
		Status:     model.ReportStatusPending,
	})
	return &types.ReviewReportResp{Id: report.Id}, nil
}
