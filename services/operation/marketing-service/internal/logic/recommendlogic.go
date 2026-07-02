package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/marketing-service/internal/model"
	"github.com/askxuan/marketing-service/internal/svc"
	"github.com/askxuan/marketing-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// recommendToType model.Recommend → types.Recommend
func recommendToType(r model.Recommend) types.Recommend {
	return types.Recommend{
		Id:        r.Id,
		Type:      r.Type,
		TargetId:  r.TargetId,
		Sort:      r.Sort,
		Status:    r.Status,
		CreatedAt: r.CreatedAt,
	}
}

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

// RecommendList C 端推荐位列表（仅返回 enabled，按 type 查询：temple/master/product）
func (l *CustomerRecommendListLogic) RecommendList(req *types.RecommendListReq) (*types.RecommendListResp, error) {
	list, total := model.ListRecommends(req.Type, model.StatusEnabled, req.Page, req.Size)
	out := make([]types.Recommend, 0, len(list))
	for _, r := range list {
		out = append(out, recommendToType(r))
	}
	return &types.RecommendListResp{
		Total: total,
		List:  out,
		Page:  req.Page,
		Size:  req.Size,
	}, nil
}

// ===== 平台 推荐位 =====

// AdminRecommendListLogic 推荐位列表（管理台）
type AdminRecommendListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminRecommendListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminRecommendListLogic {
	return &AdminRecommendListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// List 管理台推荐位列表（直接转换）
func (l *AdminRecommendListLogic) List(req *types.RecommendListReq) (*types.RecommendListResp, error) {
	list, total := model.ListRecommends(req.Type, req.Status, req.Page, req.Size)
	out := make([]types.Recommend, 0, len(list))
	for _, r := range list {
		out = append(out, recommendToType(r))
	}
	return &types.RecommendListResp{
		Total: total,
		List:  out,
		Page:  req.Page,
		Size:  req.Size,
	}, nil
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

// Update 更新推荐位，不存在返回 40404
func (l *AdminRecommendUpdateLogic) Update(req *types.RecommendUpdateReq) (*types.IdResp, error) {
	r, ok := model.UpdateRecommend(req.Id, model.Recommend{
		Type:     req.Type,
		TargetId: req.TargetId,
		Sort:     req.Sort,
		Status:   req.Status,
	})
	if !ok {
		return nil, common.NewBizError(40404, "推荐位不存在")
	}
	return &types.IdResp{Id: r.Id}, nil
}
