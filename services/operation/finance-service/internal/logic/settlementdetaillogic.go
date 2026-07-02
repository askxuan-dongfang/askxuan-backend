package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/finance-service/internal/model"
	"github.com/askxuan/finance-service/internal/svc"
	"github.com/askxuan/finance-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// SettlementDetailLogic 结算详情查询逻辑
type SettlementDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSettlementDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettlementDetailLogic {
	return &SettlementDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SettlementDetail 按 ID 查询结算单详情
func (l *SettlementDetailLogic) SettlementDetail(req *types.SettlementDetailReq) (*types.Settlement, error) {
	s, ok := model.FindSettlementByID(req.Id)
	if !ok {
		return nil, common.NewBizError(40404, "结算单不存在")
	}
	return &types.Settlement{
		Id: s.Id, SettlementNo: s.SettlementNo, SettleType: s.SettleType,
		TargetId: s.TargetId, TargetName: s.TargetName,
		PeriodStart: s.PeriodStart, PeriodEnd: s.PeriodEnd,
		OrderCount: s.OrderCount, TotalAmount: s.TotalAmount,
		CommissionRate: s.CommissionRate, CommissionAmount: s.CommissionAmount,
		SettleAmount: s.SettleAmount, Status: s.Status, CreateTime: s.CreateTime,
	}, nil
}
