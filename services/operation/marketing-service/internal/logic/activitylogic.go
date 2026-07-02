package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/marketing-service/internal/svc"
	"github.com/askxuan/marketing-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// ===== C 端 活动 =====

// CustomerActivityListLogic 活动列表
type CustomerActivityListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCustomerActivityListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CustomerActivityListLogic {
	return &CustomerActivityListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// ActivityList C 端活动列表（仅返回启用且在有效期内的）
func (l *CustomerActivityListLogic) ActivityList(req *types.ActivityListReq) (*types.ActivityListResp, error) {
	return nil, common.ErrNotImplemented
}

// ===== 平台台 活动 =====

// AdminActivityListLogic 活动列表（管理台）
type AdminActivityListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminActivityListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminActivityListLogic {
	return &AdminActivityListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminActivityListLogic) List(req *types.ActivityListReq) (*types.ActivityListResp, error) {
	return nil, common.ErrNotImplemented
}

// AdminActivityCreateLogic 创建活动
type AdminActivityCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminActivityCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminActivityCreateLogic {
	return &AdminActivityCreateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminActivityCreateLogic) Create(req *types.ActivityCreateReq) (*types.IdResp, error) {
	return nil, common.ErrNotImplemented
}

// AdminActivityUpdateLogic 更新活动
type AdminActivityUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminActivityUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminActivityUpdateLogic {
	return &AdminActivityUpdateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminActivityUpdateLogic) Update(req *types.ActivityUpdateReq) (*types.IdResp, error) {
	return nil, common.ErrNotImplemented
}
