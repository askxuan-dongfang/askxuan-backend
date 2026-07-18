package logic

import (
	"context"
	"errors"

	"github.com/askxuan/common"
	"github.com/askxuan/review-service/internal/model"
	"github.com/askxuan/review-service/internal/svc"
	"github.com/askxuan/review-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
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
	report, err := model.FindReportByID(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.NewBizError(40414, "举报不存在")
		}
		l.Errorf("查询评价举报失败: %v", err)
		return nil, common.ErrSystem
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

	// 举报状态与评价可见性必须在同一事务内提交。
	updated, err := model.HandleReport(l.ctx, report, targetStatus, req.HandleResult)
	if err != nil {
		l.Errorf("更新评价举报失败: %v", err)
		return nil, common.ErrSystem
	}
	if !updated {
		return nil, common.ErrStatusInvalid
	}

	return &types.ReportHandleResp{
		Id:     req.Id,
		Status: targetStatus,
	}, nil
}
