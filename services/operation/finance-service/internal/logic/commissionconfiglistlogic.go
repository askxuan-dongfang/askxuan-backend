package logic

import (
	"context"

	"github.com/askxuan/finance-service/internal/model"
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
	list := model.ListCommissionConfigs(req.BizType)
	resp := &types.CommissionConfigListResp{}
	for _, c := range list {
		resp.List = append(resp.List, types.CommissionConfig{
			Id: c.Id, BizType: c.BizType, Rate: c.Rate,
			Description: c.Description, UpdateTime: c.UpdateTime,
		})
	}
	return resp, nil
}
