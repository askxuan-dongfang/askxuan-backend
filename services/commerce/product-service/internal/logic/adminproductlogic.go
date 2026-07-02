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

// ===== 商城台商品列表 =====

type AdminProductListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminProductListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminProductListLogic {
	return &AdminProductListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminProductListLogic) List(req *types.AdminProductListReq) (*types.AdminProductListResp, error) {
	list, total, err := l.svcCtx.ProductModel.FindList(l.ctx, req.CategoryId, req.Keyword, req.Status, req.Page, req.Size)
	if err != nil {
		l.Errorf("查询商品列表失败: %v", err)
		return nil, common.ErrSystem
	}
	result := make([]types.Product, 0, len(list))
	for _, p := range list {
		result = append(result, toTypesProduct(p))
	}
	return &types.AdminProductListResp{Total: total, List: result, Page: req.Page, Size: req.Size}, nil
}

// ===== 商城台创建商品 =====

type AdminProductCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminProductCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminProductCreateLogic {
	return &AdminProductCreateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminProductCreateLogic) Create(req *types.AdminProductCreateReq) (*types.AdminProductCreateResp, error) {
	p, err := l.svcCtx.ProductModel.Insert(l.ctx, &model.Product{
		Name:              req.Name,
		CategoryId:        req.CategoryId,
		Description:       req.Description,
		MainImage:         req.MainImage,
		Price:             req.Price,
		MarketPrice:       req.MarketPrice,
		Stock:             req.Stock,
		Tags:              req.Tags,
		FreightTemplateId: req.FreightTemplateId,
	})
	if err != nil {
		l.Errorf("创建商品失败: %v", err)
		return nil, common.ErrSystem
	}
	return &types.AdminProductCreateResp{Id: p.Id}, nil
}

// ===== 商城台商品详情 =====

type AdminProductDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminProductDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminProductDetailLogic {
	return &AdminProductDetailLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminProductDetailLogic) Detail(req *types.AdminProductDetailReq) (*types.Product, error) {
	p, err := l.svcCtx.ProductModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrProductNotFound
		}
		l.Errorf("查询商品详情失败: %v", err)
		return nil, common.ErrSystem
	}
	resp := toTypesProduct(p)

	skus, err := l.svcCtx.ProductSkuModel.ListByProductId(l.ctx, req.Id)
	if err == nil {
		for _, s := range skus {
			resp.Skus = append(resp.Skus, toTypesSku(s))
		}
	}

	images, err := l.svcCtx.ProductImageModel.ListByProductId(l.ctx, req.Id)
	if err == nil {
		for _, img := range images {
			resp.Images = append(resp.Images, toTypesImage(img))
		}
	}

	return &resp, nil
}

// ===== 商城台更新商品 =====

type AdminProductUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminProductUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminProductUpdateLogic {
	return &AdminProductUpdateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminProductUpdateLogic) Update(req *types.AdminProductUpdateReq) (*types.Product, error) {
	err := l.svcCtx.ProductModel.Update(l.ctx, &model.Product{
		Id:                req.Id,
		Name:              req.Name,
		CategoryId:        req.CategoryId,
		Description:       req.Description,
		MainImage:         req.MainImage,
		Price:             req.Price,
		MarketPrice:       req.MarketPrice,
		Stock:             req.Stock,
		Tags:              req.Tags,
		FreightTemplateId: req.FreightTemplateId,
	})
	if err != nil {
		l.Errorf("更新商品失败: %v", err)
		return nil, common.ErrSystem
	}
	p, err := l.svcCtx.ProductModel.FindOne(l.ctx, req.Id)
	if err != nil {
		return nil, common.ErrSystem
	}
	resp := toTypesProduct(p)
	return &resp, nil
}

// ===== 商城台删除商品 =====

type AdminProductDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminProductDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminProductDeleteLogic {
	return &AdminProductDeleteLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminProductDeleteLogic) Delete(req *types.AdminProductDeleteReq) error {
	err := l.svcCtx.ProductModel.Delete(l.ctx, req.Id)
	if err != nil {
		l.Errorf("删除商品失败: %v", err)
		return common.ErrSystem
	}
	return nil
}

// ===== 商城台上下架 =====

type AdminProductStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminProductStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminProductStatusLogic {
	return &AdminProductStatusLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminProductStatusLogic) Status(req *types.AdminProductStatusReq) (*types.AdminProductStatusResp, error) {
	if req.Status != model.ProductStatusOnShelf && req.Status != model.ProductStatusOffShelf {
		return nil, common.ErrParam
	}
	err := l.svcCtx.ProductModel.UpdateStatus(l.ctx, req.Id, req.Status)
	if err != nil {
		l.Errorf("更新商品状态失败: %v", err)
		return nil, common.ErrSystem
	}
	return &types.AdminProductStatusResp{Id: req.Id, Status: req.Status}, nil
}
