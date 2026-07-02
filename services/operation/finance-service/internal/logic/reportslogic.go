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
	settlementSums := model.SumSettlementBySettleType(req.StartTime, req.EndTime)
	totalSettlement := 0.0
	for _, v := range settlementSums {
		totalSettlement += v
	}
	financeSums := model.SumByType(req.StartTime, req.EndTime)
	totalIncome := financeSums["income"]
	return &types.ReportResp{
		TotalIncome:     totalIncome,
		TotalSettlement: totalSettlement,
		TotalWithdrawal: 0,
		OrderCount:      0,
	}, nil
}
