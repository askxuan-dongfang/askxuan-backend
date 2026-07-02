package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/finance-service/internal/svc"
	"github.com/askxuan/finance-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// OverviewLogic 收入总览逻辑
type OverviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOverviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OverviewLogic {
	return &OverviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Overview 收入总览：总收入/寺院收入/法师收入/商城收入/平台抽成/待审核提现数
func (l *OverviewLogic) Overview(req *types.OverviewReq) (*types.OverviewResp, error) {
	// TODO: 聚合 settlement + withdrawal 数据计算收入总览
	return nil, common.ErrNotImplemented
}
