package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/review-service/internal/model"
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
	// 查询举报记录
	report, ok := model.FindReportByID(req.Id)
	if !ok {
		return nil, common.NewBizError(40414, "举报不存在")
	}

	// 确定目标状态
	var targetStatus string
	switch req.HandleResult {
	case "handled":
		targetStatus = model.ReportStatusHandled
	case "rejected":
		targetStatus = model.ReportStatusRejected
	default:
		return nil, common.ErrParam
	}

	// 校验状态流转是否合法
	if !model.CanTransitReport(report.Status, targetStatus) {
		return nil, common.ErrStatusInvalid
	}

	// 更新举报状态和处理结果
	model.UpdateReportStatus(req.Id, targetStatus, req.HandleResult)

	// 若处理为 handled，则同步隐藏对应评价
	if targetStatus == model.ReportStatusHandled {
		model.UpdateReviewStatus(report.ReviewId, model.ReviewStatusHidden)
	}

	return &types.ReportHandleResp{
		Id:     req.Id,
		Status: targetStatus,
	}, nil
}
