package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/marketing-service/internal/svc"
	"github.com/askxuan/marketing-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// ===== C 端 Banner =====

// CustomerBannerListLogic 首页 Banner 列表
type CustomerBannerListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCustomerBannerListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CustomerBannerListLogic {
	return &CustomerBannerListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// BannerList C 端首页 Banner 列表（仅返回启用且在有效期内的）
func (l *CustomerBannerListLogic) BannerList(req *types.BannerListReq) (*types.BannerListResp, error) {
	return nil, common.ErrNotImplemented
}

// ===== 平台台 Banner =====

// AdminBannerListLogic Banner 列表（管理台）
type AdminBannerListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminBannerListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminBannerListLogic {
	return &AdminBannerListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminBannerListLogic) List(req *types.BannerListReq) (*types.BannerListResp, error) {
	return nil, common.ErrNotImplemented
}

// AdminBannerCreateLogic 创建 Banner
type AdminBannerCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminBannerCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminBannerCreateLogic {
	return &AdminBannerCreateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminBannerCreateLogic) Create(req *types.BannerCreateReq) (*types.IdResp, error) {
	return nil, common.ErrNotImplemented
}

// AdminBannerUpdateLogic 更新 Banner
type AdminBannerUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminBannerUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminBannerUpdateLogic {
	return &AdminBannerUpdateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminBannerUpdateLogic) Update(req *types.BannerUpdateReq) (*types.IdResp, error) {
	return nil, common.ErrNotImplemented
}
