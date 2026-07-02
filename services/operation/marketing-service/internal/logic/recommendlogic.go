package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/marketing-service/internal/svc"
	"github.com/askxuan/marketing-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// ===== C 端 推荐位 =====

// CustomerRecommendListLogic 推荐位列表
type CustomerRecommendListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCustomerRecommendListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CustomerRecommendListLogic {
	return &CustomerRecommendListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// RecommendList C 端推荐位列表（按 type 查询：temple/master/product）
func (l *CustomerRecommendListLogic) RecommendList(req *types.RecommendListReq) (*types.RecommendListResp, error) {
	return nil, common.ErrNotImplemented
}

// ===== 平台台 推荐位 =====

// AdminRecommendListLogic 推荐位列表（管理台）
type AdminRecommendListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminRecommendListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminRecommendListLogic {
	return &AdminRecommendListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminRecommendListLogic) List(req *types.RecommendListReq) (*types.RecommendListResp, error) {
	return nil, common.ErrNotImplemented
}

// AdminRecommendUpdateLogic 更新推荐位
type AdminRecommendUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminRecommendUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminRecommendUpdateLogic {
	return &AdminRecommendUpdateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminRecommendUpdateLogic) Update(req *types.RecommendUpdateReq) (*types.IdResp, error) {
	return nil, common.ErrNotImplemented
}
