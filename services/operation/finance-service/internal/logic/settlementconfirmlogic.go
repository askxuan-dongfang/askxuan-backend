package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/finance-service/internal/model"
	"github.com/askxuan/finance-service/internal/svc"
	"github.com/askxuan/finance-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// SettlementConfirmLogic 确认结算逻辑
type SettlementConfirmLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSettlementConfirmLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SettlementConfirmLogic {
	return &SettlementConfirmLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// SettlementConfirm 确认结算（status: pending → confirmed）
func (l *SettlementConfirmLogic) SettlementConfirm(req *types.SettlementConfirmReq) (*types.SettlementConfirmResp, error) {
	s, ok := model.FindSettlementByID(req.Id)
	if !ok {
		return nil, common.NewBizError(40404, "结算单不存在")
	}
	if !model.CanTransitSettlement(s.Status, model.SettlementConfirmed) {
		return nil, common.ErrStatusInvalid
	}
	if !model.UpdateSettlementStatus(req.Id, model.SettlementConfirmed) {
		return nil, common.ErrStatusInvalid
	}
	return &types.SettlementConfirmResp{Id: req.Id, Status: model.SettlementConfirmed}, nil
}
