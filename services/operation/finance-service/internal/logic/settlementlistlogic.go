package logic

import (
	"context"

	"github.com/askxuan/finance-service/internal/model"
	"github.com/askxuan/finance-service/internal/svc"
	"github.com/askxuan/finance-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// SettlementListLogic 结算列表查询逻辑
type SettlementListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSettlementListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettlementListLogic {
	return &SettlementListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SettlementList 结算列表，支持按 settleType/status 筛选 + 分页
func (l *SettlementListLogic) SettlementList(req *types.SettlementListReq) (*types.SettlementListResp, error) {
	list, total := model.ListSettlements(req.SettleType, req.Status, req.Page, req.Size)
	resp := &types.SettlementListResp{Total: total, Page: req.Page, Size: req.Size}
	for _, s := range list {
		resp.List = append(resp.List, types.Settlement{
			Id: s.Id, SettlementNo: s.SettlementNo, SettleType: s.SettleType,
			TargetId: s.TargetId, TargetName: s.TargetName,
			PeriodStart: s.PeriodStart, PeriodEnd: s.PeriodEnd,
			OrderCount: s.OrderCount, TotalAmount: s.TotalAmount,
			CommissionRate: s.CommissionRate, CommissionAmount: s.CommissionAmount,
			SettleAmount: s.SettleAmount, Status: s.Status, CreateTime: s.CreateTime,
		})
	}
	return resp, nil
}
