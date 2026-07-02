package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/finance-service/internal/svc"
	"github.com/askxuan/finance-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// CommissionConfigListLogic 抽成配置列表逻辑
type CommissionConfigListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCommissionConfigListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CommissionConfigListLogic {
	return &CommissionConfigListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// CommissionConfigList 抽成配置列表，支持按 bizType 筛选
func (l *CommissionConfigListLogic) CommissionConfigList(req *types.CommissionConfigListReq) (*types.CommissionConfigListResp, error) {
	// TODO: 调用 model.ListCommissionConfigs 查询
	return nil, common.ErrNotImplemented
}
