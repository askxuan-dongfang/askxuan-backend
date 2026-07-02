package logic

import (
	"context"

	"github.com/askxuan/common"
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
	// TODO: 调用 model.FindSettlementByID 查询
	return nil, common.ErrNotImplemented
}
