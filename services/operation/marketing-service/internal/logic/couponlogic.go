package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/marketing-service/internal/model"
	"github.com/askxuan/marketing-service/internal/svc"
	"github.com/askxuan/marketing-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// couponToType model.Coupon → types.Coupon
func couponToType(c model.Coupon) types.Coupon {
	return types.Coupon{
		Id:            c.Id,
		CouponNo:      c.CouponNo,
		Name:          c.Name,
		Type:          c.Type,
		Value:         c.Value,
		MinAmount:     c.MinAmount,
		CategoryId:    c.CategoryId,
		StartTime:     c.StartTime,
		EndTime:       c.EndTime,
		TotalCount:    c.TotalCount,
		ReceivedCount: c.ReceivedCount,
		Status:        c.Status,
		CreatedAt:     c.CreatedAt,
	}
}

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

// CouponList 可领取优惠券列表（启用 + 有库存 + 在有效期内）
func (l *CustomerCouponListLogic) CouponList(req *types.CouponListReq) (*types.CouponListResp, error) {
	// C 端取全部启用的优惠券再过滤（model 层 size 上限 100，取最大值）
	list, _ := model.ListCoupons(model.StatusEnabled, req.Type, 1, 100)
	// 过滤：有库存（ReceivedCount < TotalCount）且当前时间在 StartTime~EndTime 之间
	filtered := make([]types.Coupon, 0, len(list))
	for _, c := range list {
		if c.ReceivedCount >= c.TotalCount {
			continue
		}
		if !inTimeRange(c.StartTime, c.EndTime) {
			continue
		}
		filtered = append(filtered, couponToType(c))
	}
	// 手动分页（先过滤再分页）
	page := req.Page
	size := req.Size
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	total := int64(len(filtered))
	start := (page - 1) * size
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + size
	if end > len(filtered) {
		end = len(filtered)
	}
	return &types.CouponListResp{
		Total: total,
		List:  filtered[start:end],
		Page:  page,
		Size:  size,
	}, nil
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

// Receive 领取优惠券（校验库存 + 写领取记录），回填冗余字段
func (l *CustomerCouponReceiveLogic) Receive(req *types.CouponReceiveReq) (*types.CouponRecord, error) {
	record, err := model.ReceiveCoupon(req.Id, req.UserId)
	if err != nil {
		return nil, common.NewBizError(40009, err.Error())
	}
	// 回填冗余字段（Name/Type/Value/MinAmount/EndTime）
	coupon, _ := model.FindCouponByID(req.Id)
	return &types.CouponRecord{
		Id:        record.Id,
		CouponId:  record.CouponId,
		CouponNo:  record.CouponNo,
		UserId:    record.UserId,
		Status:    record.Status,
		OrderNo:   record.OrderNo,
		UseTime:   record.UseTime,
		CreatedAt: record.CreatedAt,
		Name:      coupon.Name,
		Type:      coupon.Type,
		Value:     coupon.Value,
		MinAmount: coupon.MinAmount,
		EndTime:   coupon.EndTime,
	}, nil
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

// MyCoupon 我的优惠券列表（unused/used/expired），逐条回填冗余字段
func (l *CustomerMyCouponLogic) MyCoupon(req *types.MyCouponReq) (*types.MyCouponResp, error) {
	list, total := model.ListMyCoupons(req.UserId, req.Status, req.Page, req.Size)
	out := make([]types.CouponRecord, 0, len(list))
	for _, r := range list {
		// 回填冗余字段
		coupon, _ := model.FindCouponByID(r.CouponId)
		out = append(out, types.CouponRecord{
			Id:        r.Id,
			CouponId:  r.CouponId,
			CouponNo:  r.CouponNo,
			UserId:    r.UserId,
			Status:    r.Status,
			OrderNo:   r.OrderNo,
			UseTime:   r.UseTime,
			CreatedAt: r.CreatedAt,
			Name:      coupon.Name,
			Type:      coupon.Type,
			Value:     coupon.Value,
			MinAmount: coupon.MinAmount,
			EndTime:   coupon.EndTime,
		})
	}
	return &types.MyCouponResp{
		Total: total,
		List:  out,
		Page:  req.Page,
		Size:  req.Size,
	}, nil
}

// ===== 平台 优惠券 =====

// AdminCouponListLogic 优惠券列表（管理台）
type AdminCouponListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminCouponListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminCouponListLogic {
	return &AdminCouponListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// List 管理台优惠券列表（直接转换）
func (l *AdminCouponListLogic) List(req *types.CouponListReq) (*types.CouponListResp, error) {
	list, total := model.ListCoupons(req.Status, req.Type, req.Page, req.Size)
	out := make([]types.Coupon, 0, len(list))
	for _, c := range list {
		out = append(out, couponToType(c))
	}
	return &types.CouponListResp{
		Total: total,
		List:  out,
		Page:  req.Page,
		Size:  req.Size,
	}, nil
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

// Create 创建优惠券
func (l *AdminCouponCreateLogic) Create(req *types.CouponCreateReq) (*types.IdResp, error) {
	c := model.InsertCoupon(model.Coupon{
		Name:       req.Name,
		Type:       req.Type,
		Value:      req.Value,
		MinAmount:  req.MinAmount,
		CategoryId: req.CategoryId,
		StartTime:  req.StartTime,
		EndTime:    req.EndTime,
		TotalCount: req.TotalCount,
	})
	return &types.IdResp{Id: c.Id}, nil
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

// Update 更新优惠券，不存在返回 40404
func (l *AdminCouponUpdateLogic) Update(req *types.CouponUpdateReq) (*types.IdResp, error) {
	c, ok := model.UpdateCoupon(req.Id, model.Coupon{
		Name:       req.Name,
		Type:       req.Type,
		Value:      req.Value,
		MinAmount:  req.MinAmount,
		CategoryId: req.CategoryId,
		StartTime:  req.StartTime,
		EndTime:    req.EndTime,
		TotalCount: req.TotalCount,
		Status:     req.Status,
	})
	if !ok {
		return nil, common.NewBizError(40404, "优惠券不存在")
	}
	return &types.IdResp{Id: c.Id}, nil
}
