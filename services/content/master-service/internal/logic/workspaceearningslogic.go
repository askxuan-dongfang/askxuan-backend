package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/master-service/internal/model"
	"github.com/askxuan/master-service/internal/svc"
	"github.com/askxuan/master-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// ============ 法师工作台 - 收益 Logic ============

// WorkspaceEarningsSummaryLogic 收益汇总
type WorkspaceEarningsSummaryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWorkspaceEarningsSummaryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspaceEarningsSummaryLogic {
	return &WorkspaceEarningsSummaryLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// WorkspaceEarningsSummary 法师收益汇总
func (l *WorkspaceEarningsSummaryLogic) WorkspaceEarningsSummary(req *types.EarningsSummaryReq) (*types.EarningsSummaryResp, error) {
	masterCode, err := currentMasterCode(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}

	summary, err := model.GetEarningsSummary(l.ctx, masterCode)
	if err != nil {
		l.Errorf("查询收益汇总失败: %v", err)
		return nil, common.ErrSystem
	}

	trend := make([]types.EarningsTrendItem, 0, len(summary.Trend))
	for _, t := range summary.Trend {
		trend = append(trend, types.EarningsTrendItem{
			Month:  t.Month,
			Amount: t.Amount,
		})
	}

	return &types.EarningsSummaryResp{
		MonthIncome:  summary.MonthIncome,
		TotalIncome:  summary.TotalIncome,
		Withdrawable: summary.Withdrawable,
		Trend:        trend,
	}, nil
}

// WorkspaceEarningsDetailsLogic 收益明细列表
type WorkspaceEarningsDetailsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWorkspaceEarningsDetailsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspaceEarningsDetailsLogic {
	return &WorkspaceEarningsDetailsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// WorkspaceEarningsDetails 法师收益明细列表
func (l *WorkspaceEarningsDetailsLogic) WorkspaceEarningsDetails(req *types.EarningsDetailReq) (*types.EarningsDetailResp, error) {
	masterCode, err := currentMasterCode(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}

	page := req.Page
	size := req.Size
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	if masterCode == "" {
		return nil, common.ErrUnauthorized
	}

	list, total, err := model.ListEarnings(l.ctx, masterCode, req.ServiceType, page, size)
	if err != nil {
		l.Errorf("查询收益明细失败: %v", err)
		return nil, common.ErrSystem
	}

	out := make([]types.EarningsDetailItem, 0, len(list))
	for _, e := range list {
		out = append(out, types.EarningsDetailItem{
			Id:           e.Id,
			Date:         e.Date,
			ServiceType:  e.ServiceType,
			UserName:     e.UserName,
			Amount:       e.Amount,
			SettleStatus: e.SettleStatus,
		})
	}

	return &types.EarningsDetailResp{
		Total: total,
		List:  out,
		Page:  page,
		Size:  size,
	}, nil
}
