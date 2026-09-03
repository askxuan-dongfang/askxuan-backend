package logic

import (
	"context"

	"github.com/askxuan/finance-service/internal/model"
	"github.com/askxuan/finance-service/internal/svc"
	"github.com/askxuan/finance-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// OverviewLogic 收入总览逻辑
type OverviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOverviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OverviewLogic {
	return &OverviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Overview 收入总览：总收入/寺院收入/法师收入/商城收入/平台抽成/待审核提现数
func (l *OverviewLogic) Overview(req *types.OverviewReq) (*types.OverviewResp, error) {
	settlementSums, err := model.QuerySettlementNetByType(l.ctx, req.StartTime, req.EndTime)
	if err != nil {
		l.Errorf("查询结算净额失败: %v", err)
		return nil, err
	}
	templeIncome := settlementSums[model.SettleTypeTemple]
	masterIncome := settlementSums[model.SettleTypeMaster]
	shopIncome := settlementSums[model.SettleTypeShop]
	financial, err := model.QueryFinancialReport(l.ctx, req.StartTime, req.EndTime)
	if err != nil {
		l.Errorf("查询总账收款失败: %v", err)
		return nil, err
	}
	totalIncome := financial.GrossIncome - financial.RefundAmount
	commissionIncome, err := model.QueryCommission(l.ctx, req.StartTime, req.EndTime)
	if err != nil {
		l.Errorf("查询平台抽成失败: %v", err)
		return nil, err
	}
	pendingWithdraw := int(model.CountWithdrawalByStatus(model.WithdrawalPending))
	return &types.OverviewResp{
		TotalIncome:      totalIncome,
		TempleIncome:     templeIncome,
		MasterIncome:     masterIncome,
		ShopIncome:       shopIncome,
		CommissionIncome: commissionIncome,
		PendingWithdraw:  pendingWithdraw,
	}, nil
}
