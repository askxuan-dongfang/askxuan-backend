package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/review-service/internal/model"
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
	list, total, err := model.ListReports(l.ctx, req.Status, req.Page, req.Size)
	if err != nil {
		l.Errorf("查询评价举报失败: %v", err)
		return nil, common.ErrSystem
	}

	result := make([]types.ReviewReport, 0, len(list))
	for _, r := range list {
		result = append(result, types.ReviewReport{
			Id:           r.Id,
			ReviewId:     r.ReviewId,
			ReporterId:   r.ReporterId,
			Reason:       r.Reason,
			Status:       r.Status,
			HandleResult: r.HandleResult,
			CreateTime:   r.CreateTime,
		})
	}

	return &types.ReportListResp{
		Total: total,
		List:  result,
		Page:  req.Page,
		Size:  req.Size,
	}, nil
}
