package logic

import (
	"context"

	"github.com/askxuan/common"
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
	// TODO: 校验 rate 范围 + 更新 commission_config
	return nil, common.ErrNotImplemented
}
