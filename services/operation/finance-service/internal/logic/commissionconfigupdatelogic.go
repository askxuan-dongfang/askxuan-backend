package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/finance-service/internal/model"
	"github.com/askxuan/finance-service/internal/svc"
	"github.com/askxuan/finance-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// CommissionConfigUpdateLogic 修改抽成比例逻辑
type CommissionConfigUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCommissionConfigUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CommissionConfigUpdateLogic {
	return &CommissionConfigUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// CommissionConfigUpdate 修改抽成比例（仅平台超管）
func (l *CommissionConfigUpdateLogic) CommissionConfigUpdate(req *types.CommissionConfigUpdateReq) (*types.CommissionConfigUpdateResp, error) {
	if req.Rate < 0 || req.Rate > 1 {
		return nil, common.ErrParam
	}
	err := model.UpdateCommissionConfig(req.Id, req.Rate, req.Description)
	if err != nil {
		return nil, common.NewBizError(40404, "配置不存在")
	}
	return &types.CommissionConfigUpdateResp{Id: req.Id}, nil
}
