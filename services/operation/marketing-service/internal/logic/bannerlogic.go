package logic

import (
	"context"
	"time"

	"github.com/askxuan/common"
	"github.com/askxuan/marketing-service/internal/model"
	"github.com/askxuan/marketing-service/internal/svc"
	"github.com/askxuan/marketing-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// inTimeRange 判断当前时间是否落在 [start, end] 区间内
// 支持格式 "2006-01-02 15:04:05" 与 "2006-01-02"；解析失败时按"在范围内"处理（不过滤）
func inTimeRange(start, end string) bool {
	now := time.Now()
	const layout = "2006-01-02 15:04:05"
	const layoutShort = "2006-01-02"
	if start != "" {
		t, err := time.ParseInLocation(layout, start, time.Local)
		if err != nil {
			t, err = time.ParseInLocation(layoutShort, start, time.Local)
		}
		if err == nil && now.Before(t) {
			return false
		}
	}
	if end != "" {
		t, err := time.ParseInLocation(layout, end, time.Local)
		if err != nil {
			t, err = time.ParseInLocation(layoutShort, end, time.Local)
		}
		if err == nil && now.After(t) {
			return false
		}
	}
	return true
}

// bannerToType model.Banner → types.Banner
func bannerToType(b model.Banner) types.Banner {
	return types.Banner{
		Id:        b.Id,
		Title:     b.Title,
		ImageUrl:  b.ImageUrl,
		LinkType:  b.LinkType,
		LinkValue: b.LinkValue,
		Sort:      b.Sort,
		Status:    b.Status,
		StartTime: b.StartTime,
		EndTime:   b.EndTime,
		CreatedAt: b.CreatedAt,
	}
}

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
	// C 端仅返回 enabled 且在有效期内的 Banner
	list, _ := model.ListBanners(model.StatusEnabled, req.Page, req.Size)
	// 过滤当前时间在 StartTime~EndTime 之间，过滤后 total 重新计算
	filtered := make([]types.Banner, 0, len(list))
	for _, b := range list {
		if !inTimeRange(b.StartTime, b.EndTime) {
			continue
		}
		filtered = append(filtered, bannerToType(b))
	}
	return &types.BannerListResp{
		Total: int64(len(filtered)),
		List:  filtered,
		Page:  req.Page,
		Size:  req.Size,
	}, nil
}

// ===== 平台 Banner =====

// AdminBannerListLogic Banner 列表（管理台）
type AdminBannerListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminBannerListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminBannerListLogic {
	return &AdminBannerListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// List 管理台 Banner 列表（直接转换，不过滤有效期）
func (l *AdminBannerListLogic) List(req *types.BannerListReq) (*types.BannerListResp, error) {
	list, total := model.ListBanners(req.Status, req.Page, req.Size)
	out := make([]types.Banner, 0, len(list))
	for _, b := range list {
		out = append(out, bannerToType(b))
	}
	return &types.BannerListResp{
		Total: total,
		List:  out,
		Page:  req.Page,
		Size:  req.Size,
	}, nil
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

// Create 创建 Banner
func (l *AdminBannerCreateLogic) Create(req *types.BannerCreateReq) (*types.IdResp, error) {
	b := model.InsertBanner(model.Banner{
		Title:     req.Title,
		ImageUrl:  req.ImageUrl,
		LinkType:  req.LinkType,
		LinkValue: req.LinkValue,
		Sort:      req.Sort,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
	})
	return &types.IdResp{Id: b.Id}, nil
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

// Update 更新 Banner，不存在返回 40404
func (l *AdminBannerUpdateLogic) Update(req *types.BannerUpdateReq) (*types.IdResp, error) {
	b, ok := model.UpdateBanner(req.Id, model.Banner{
		Title:     req.Title,
		ImageUrl:  req.ImageUrl,
		LinkType:  req.LinkType,
		LinkValue: req.LinkValue,
		Sort:      req.Sort,
		Status:    req.Status,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
	})
	if !ok {
		return nil, common.NewBizError(40404, "Banner不存在")
	}
	return &types.IdResp{Id: b.Id}, nil
}
