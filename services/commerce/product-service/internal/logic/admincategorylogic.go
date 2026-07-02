package logic

import (
	"context"
	"errors"

	"github.com/askxuan/common"
	"github.com/askxuan/product-service/internal/model"
	"github.com/askxuan/product-service/internal/svc"
	"github.com/askxuan/product-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ===== 商城台分类列表 =====

type AdminCategoryListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminCategoryListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminCategoryListLogic {
	return &AdminCategoryListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminCategoryListLogic) List(req *types.AdminCategoryListReq) (*types.AdminCategoryListResp, error) {
	var list []*model.ProductCategory
	var err error
	if req.ParentId > 0 {
		list, err = l.svcCtx.ProductCategoryModel.ListByParentId(l.ctx, req.ParentId)
	} else {
		list, err = l.svcCtx.ProductCategoryModel.ListAll(l.ctx)
	}
	if err != nil {
		l.Errorf("查询分类列表失败: %v", err)
		return nil, common.ErrSystem
	}
	result := make([]types.ProductCategory, 0, len(list))
	for _, c := range list {
		result = append(result, types.ProductCategory{
			Id:       c.Id,
			ParentId: c.ParentId,
			Name:     c.Name,
			Level:    c.Level,
			Sort:     c.Sort,
		})
	}
	return &types.AdminCategoryListResp{Total: int64(len(result)), List: result, Page: req.Page, Size: req.Size}, nil
}

// ===== 商城台创建分类 =====

type AdminCategoryCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminCategoryCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminCategoryCreateLogic {
	return &AdminCategoryCreateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminCategoryCreateLogic) Create(req *types.AdminCategoryCreateReq) (*types.AdminCategoryCreateResp, error) {
	c, err := l.svcCtx.ProductCategoryModel.Insert(l.ctx, &model.ProductCategory{
		ParentId: req.ParentId,
		Name:     req.Name,
		Level:    req.Level,
		Sort:     req.Sort,
	})
	if err != nil {
		l.Errorf("创建分类失败: %v", err)
		return nil, common.ErrSystem
	}
	return &types.AdminCategoryCreateResp{Id: c.Id}, nil
}

// ===== 商城台更新分类 =====

type AdminCategoryUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminCategoryUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminCategoryUpdateLogic {
	return &AdminCategoryUpdateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminCategoryUpdateLogic) Update(req *types.AdminCategoryUpdateReq) (*types.ProductCategory, error) {
	err := l.svcCtx.ProductCategoryModel.Update(l.ctx, &model.ProductCategory{
		Id:       req.Id,
		ParentId: req.ParentId,
		Name:     req.Name,
		Level:    req.Level,
		Sort:     req.Sort,
	})
	if err != nil {
		l.Errorf("更新分类失败: %v", err)
		return nil, common.ErrSystem
	}
	c, err := l.svcCtx.ProductCategoryModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrProductNotFound
		}
		return nil, common.ErrSystem
	}
	return &types.ProductCategory{
		Id:       c.Id,
		ParentId: c.ParentId,
		Name:     c.Name,
		Level:    c.Level,
		Sort:     c.Sort,
	}, nil
}

// ===== 商城台删除分类 =====

type AdminCategoryDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminCategoryDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminCategoryDeleteLogic {
	return &AdminCategoryDeleteLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminCategoryDeleteLogic) Delete(req *types.AdminCategoryDeleteReq) error {
	// 校验无子分类
	children, err := l.svcCtx.ProductCategoryModel.ListByParentId(l.ctx, req.Id)
	if err != nil {
		l.Errorf("查询子分类失败: %v", err)
		return common.ErrSystem
	}
	if len(children) > 0 {
		return common.ErrStatusInvalid
	}
	err = l.svcCtx.ProductCategoryModel.Delete(l.ctx, req.Id)
	if err != nil {
		l.Errorf("删除分类失败: %v", err)
		return common.ErrSystem
	}
	return nil
}
