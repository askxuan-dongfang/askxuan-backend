package logic

import (
	"context"

	"github.com/askxuan/common"
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
	// TODO: 调用 model.ListSettlements 查询
	return nil, common.ErrNotImplemented
}
