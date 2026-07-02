package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/marketing-service/internal/model"
	"github.com/askxuan/marketing-service/internal/svc"
	"github.com/askxuan/marketing-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// activityToType model.Activity → types.Activity
func activityToType(a model.Activity) types.Activity {
	return types.Activity{
		Id:        a.Id,
		Name:      a.Name,
		Type:      a.Type,
		StartTime: a.StartTime,
		EndTime:   a.EndTime,
		Config:    a.Config,
		Status:    a.Status,
		CreatedAt: a.CreatedAt,
	}
}

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
	// C 端仅返回 enabled 且在有效期内的活动
	list, _ := model.ListActivities(model.StatusEnabled, req.Type, req.Page, req.Size)
	// 过滤当前时间在 StartTime~EndTime 之间，过滤后 total 重新计算
	filtered := make([]types.Activity, 0, len(list))
	for _, a := range list {
		if !inTimeRange(a.StartTime, a.EndTime) {
			continue
		}
		filtered = append(filtered, activityToType(a))
	}
	return &types.ActivityListResp{
		Total: int64(len(filtered)),
		List:  filtered,
		Page:  req.Page,
		Size:  req.Size,
	}, nil
}

// ===== 平台 活动 =====

// AdminActivityListLogic 活动列表（管理台）
type AdminActivityListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminActivityListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminActivityListLogic {
	return &AdminActivityListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// List 管理台活动列表（直接转换，不过滤有效期）
func (l *AdminActivityListLogic) List(req *types.ActivityListReq) (*types.ActivityListResp, error) {
	list, total := model.ListActivities(req.Status, req.Type, req.Page, req.Size)
	out := make([]types.Activity, 0, len(list))
	for _, a := range list {
		out = append(out, activityToType(a))
	}
	return &types.ActivityListResp{
		Total: total,
		List:  out,
		Page:  req.Page,
		Size:  req.Size,
	}, nil
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

// Create 创建活动
func (l *AdminActivityCreateLogic) Create(req *types.ActivityCreateReq) (*types.IdResp, error) {
	a := model.InsertActivity(model.Activity{
		Name:      req.Name,
		Type:      req.Type,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		Config:    req.Config,
	})
	return &types.IdResp{Id: a.Id}, nil
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

// Update 更新活动，不存在返回 40404
func (l *AdminActivityUpdateLogic) Update(req *types.ActivityUpdateReq) (*types.IdResp, error) {
	a, ok := model.UpdateActivity(req.Id, model.Activity{
		Name:      req.Name,
		Type:      req.Type,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		Config:    req.Config,
		Status:    req.Status,
	})
	if !ok {
		return nil, common.NewBizError(40404, "活动不存在")
	}
	return &types.IdResp{Id: a.Id}, nil
}
