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

// ===== 商城台添加SKU =====

type AdminSkuCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminSkuCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminSkuCreateLogic {
	return &AdminSkuCreateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminSkuCreateLogic) Create(req *types.AdminSkuCreateReq) (*types.AdminSkuCreateResp, error) {
	// 校验商品存在
	_, err := l.svcCtx.ProductModel.FindOne(l.ctx, req.ProductId)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrProductNotFound
		}
		l.Errorf("查询商品失败: %v", err)
		return nil, common.ErrSystem
	}
	s, err := l.svcCtx.ProductSkuModel.Insert(l.ctx, &model.ProductSku{
		ProductId: req.ProductId,
		SpecName:  req.SpecName,
		SpecValue: req.SpecValue,
		Price:     req.Price,
		Stock:     req.Stock,
		SkuNo:     req.SkuNo,
	})
	if err != nil {
		l.Errorf("创建SKU失败: %v", err)
		return nil, common.ErrSystem
	}
	return &types.AdminSkuCreateResp{Id: s.Id}, nil
}

// ===== 商城台更新SKU =====

type AdminSkuUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminSkuUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminSkuUpdateLogic {
	return &AdminSkuUpdateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminSkuUpdateLogic) Update(req *types.AdminSkuUpdateReq) (*types.ProductSku, error) {
	err := l.svcCtx.ProductSkuModel.Update(l.ctx, &model.ProductSku{
		Id:        req.SkuId,
		ProductId: req.ProductId,
		SpecName:  req.SpecName,
		SpecValue: req.SpecValue,
		Price:     req.Price,
		Stock:     req.Stock,
		SkuNo:     req.SkuNo,
	})
	if err != nil {
		l.Errorf("更新SKU失败: %v", err)
		return nil, common.ErrSystem
	}
	s, err := l.svcCtx.ProductSkuModel.FindOne(l.ctx, req.SkuId)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrProductNotFound
		}
		return nil, common.ErrSystem
	}
	return &types.ProductSku{
		Id:        s.Id,
		ProductId: s.ProductId,
		SpecName:  s.SpecName,
		SpecValue: s.SpecValue,
		Price:     s.Price,
		Stock:     s.Stock,
		SkuNo:     s.SkuNo,
	}, nil
}
