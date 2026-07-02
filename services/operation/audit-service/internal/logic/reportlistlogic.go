package logic

import (
	"context"

	"github.com/askxuan/audit-service/internal/model"
	"github.com/askxuan/audit-service/internal/svc"
	"github.com/askxuan/audit-service/internal/types"

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

// ReportList 举报列表，支持按 targetType/status 筛选 + 分页
func (l *ReportListLogic) ReportList(req *types.ReportListReq) (*types.ReportListResp, error) {
	list, total := model.ListReports(req.TargetType, req.Status, req.Page, req.Size)
	// 转换为 []types.Report
	out := make([]types.Report, 0, len(list))
	for _, r := range list {
		out = append(out, types.Report{
			Id:           r.Id,
			ReporterId:   r.ReporterId,
			TargetType:   r.TargetType,
			TargetId:     r.TargetId,
			Reason:       r.Reason,
			EvidenceUrls: r.EvidenceUrls,
			Status:       r.Status,
			HandlerId:    r.HandlerId,
			HandleResult: r.HandleResult,
			CreateTime:   r.CreateTime,
		})
	}
	return &types.ReportListResp{
		Total: total,
		List:  out,
		Page:  req.Page,
		Size:  req.Size,
	}, nil
}
