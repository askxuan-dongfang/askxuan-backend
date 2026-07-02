package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/marketing-service/internal/svc"
	"github.com/askxuan/marketing-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// ===== C 端 优惠券 =====

// CustomerCouponListLogic 可领取优惠券列表
type CustomerCouponListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCustomerCouponListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CustomerCouponListLogic {
	return &CustomerCouponListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// CouponList 可领取优惠券列表（启用且有库存）
func (l *CustomerCouponListLogic) CouponList(req *types.CouponListReq) (*types.CouponListResp, error) {
	return nil, common.ErrNotImplemented
}

// CustomerCouponReceiveLogic 领取优惠券
type CustomerCouponReceiveLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCustomerCouponReceiveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CustomerCouponReceiveLogic {
	return &CustomerCouponReceiveLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// Receive 领取优惠券（校验库存 + 写领取记录）
func (l *CustomerCouponReceiveLogic) Receive(req *types.CouponReceiveReq) (*types.CouponRecord, error) {
	return nil, common.ErrNotImplemented
}

// CustomerMyCouponLogic 我的优惠券
type CustomerMyCouponLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCustomerMyCouponLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CustomerMyCouponLogic {
	return &CustomerMyCouponLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// MyCoupon 我的优惠券列表（unused/used/expired）
func (l *CustomerMyCouponLogic) MyCoupon(req *types.MyCouponReq) (*types.MyCouponResp, error) {
	return nil, common.ErrNotImplemented
}

// ===== 平台台 优惠券 =====

// AdminCouponListLogic 优惠券列表（管理台）
type AdminCouponListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminCouponListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminCouponListLogic {
	return &AdminCouponListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminCouponListLogic) List(req *types.CouponListReq) (*types.CouponListResp, error) {
	return nil, common.ErrNotImplemented
}

// AdminCouponCreateLogic 创建优惠券
type AdminCouponCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminCouponCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminCouponCreateLogic {
	return &AdminCouponCreateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminCouponCreateLogic) Create(req *types.CouponCreateReq) (*types.IdResp, error) {
	return nil, common.ErrNotImplemented
}

// AdminCouponUpdateLogic 更新优惠券
type AdminCouponUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminCouponUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminCouponUpdateLogic {
	return &AdminCouponUpdateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminCouponUpdateLogic) Update(req *types.CouponUpdateReq) (*types.IdResp, error) {
	return nil, common.ErrNotImplemented
}
