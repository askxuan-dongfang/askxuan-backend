package logic

import (
	"context"

	"github.com/askxuan/audit-service/internal/model"
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
	// 查询举报记录
	r, ok := model.FindReportByID(req.Id)
	if !ok {
		return nil, common.NewBizError(40404, "举报记录不存在")
	}
	// 根据处理结果映射目标状态
	var targetStatus string
	switch req.HandleResult {
	case "handled":
		targetStatus = model.ReportStatusHandled
	case "rejected":
		targetStatus = model.ReportStatusRejected
	default:
		return nil, common.ErrParam
	}
	// 校验状态流转
	if !model.CanTransitReport(r.Status, targetStatus) {
		return nil, common.ErrStatusInvalid
	}
	// 更新举报记录
	if !model.UpdateReport(req.Id, targetStatus, req.HandlerId, req.HandleResult) {
		return nil, common.ErrStatusInvalid
	}
	return &types.ReportHandleResp{Id: req.Id, Status: targetStatus}, nil
}
