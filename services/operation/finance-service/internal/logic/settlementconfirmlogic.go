package logic

import (
	"context"

	"github.com/askxuan/common"
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
	// TODO: 校验状态流转 CanTransitSettlement + 更新状态
	return nil, common.ErrNotImplemented
}
