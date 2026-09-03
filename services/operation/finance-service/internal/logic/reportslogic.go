package logic

import (
	"context"

	"github.com/askxuan/finance-service/internal/model"
	"github.com/askxuan/finance-service/internal/svc"
	"github.com/askxuan/finance-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// ReportsLogic 财务报表逻辑
type ReportsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReportsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReportsLogic {
	return &ReportsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Reports 财务报表，支持按时间范围 + 类型筛选
func (l *ReportsLogic) Reports(req *types.ReportReq) (*types.ReportResp, error) {
	report, err := model.QueryFinancialReport(l.ctx, req.StartTime, req.EndTime)
	if err != nil {
		l.Errorf("查询权威财务报表失败: %v", err)
		return nil, err
	}
	return &types.ReportResp{
		TotalIncome:     report.GrossIncome - report.RefundAmount,
		TotalSettlement: report.SettlementNet,
		TotalWithdrawal: report.WithdrawalPaid,
		OrderCount:      report.OrderCount,
	}, nil
}
