package logic

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"

	"github.com/askxuan/common"
	"github.com/askxuan/product-service/internal/model"
	"github.com/askxuan/product-service/internal/svc"
	"github.com/askxuan/product-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ===== C端商品列表 =====

type CustomerProductListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCustomerProductListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CustomerProductListLogic {
	return &CustomerProductListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CustomerProductListLogic) List(req *types.CustomerProductListReq) (*types.CustomerProductListResp, error) {
	list, total, err := l.svcCtx.ProductModel.FindList(l.ctx, req.CategoryId, req.Keyword, model.ProductStatusOnShelf, req.Page, req.Size)
	if err != nil {
		l.Errorf("查询商品列表失败: %v", err)
		return nil, common.ErrSystem
	}
	result := make([]types.Product, 0, len(list))
	for _, p := range list {
		result = append(result, toTypesProduct(p))
	}
	return &types.CustomerProductListResp{Total: total, List: result, Page: req.Page, Size: req.Size}, nil
}

// ===== C端商品详情 =====

type CustomerProductDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCustomerProductDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CustomerProductDetailLogic {
	return &CustomerProductDetailLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CustomerProductDetailLogic) Detail(req *types.CustomerProductDetailReq) (*types.Product, error) {
	cacheKey := "product:detail:" + strconv.FormatInt(req.Id, 10)
	// 尝试命中缓存
	if cached, _ := l.svcCtx.Redis.Get(cacheKey); cached != "" {
		var p types.Product
		if err := json.Unmarshal([]byte(cached), &p); err == nil {
			return &p, nil
		}
	}

	p, err := l.svcCtx.ProductModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrProductNotFound
		}
		l.Errorf("查询商品详情失败: %v", err)
		return nil, common.ErrSystem
	}
	resp := toTypesProduct(p)
	// 查 SKU
	skus, err := l.svcCtx.ProductSkuModel.ListByProductId(l.ctx, req.Id)
	if err == nil {
		for _, s := range skus {
			resp.Skus = append(resp.Skus, toTypesSku(s))
		}
	}

	// 查图片
	images, err := l.svcCtx.ProductImageModel.ListByProductId(l.ctx, req.Id)
	if err == nil {
		for _, img := range images {
			resp.Images = append(resp.Images, toTypesImage(img))
		}
	}

	// 回写缓存（10 分钟）
	if jsonBytes, mErr := json.Marshal(resp); mErr == nil {
		_ = l.svcCtx.Redis.Setex(cacheKey, string(jsonBytes), 600)
	}

	return &resp, nil
}

// ===== C端诉求聚合 =====

type CustomerIntentionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCustomerIntentionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CustomerIntentionLogic {
	return &CustomerIntentionLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CustomerIntentionLogic) List(req *types.CustomerIntentionReq) (*types.CustomerIntentionResp, error) {
	if req.Code != "" {
		if !model.IsValidIntentCode(req.Code) {
			return nil, common.ErrParamInvalid
		}
		if _, err := l.svcCtx.IntentionModel.FindTag(l.ctx, req.Code, true); err != nil {
			return nil, common.ErrParamInvalid
		}
	}
	if req.Page < 1 {
		req.Page = 1
	}
	if req.Size < 1 || req.Size > 100 {
		req.Size = 20
	}
	tags, err := l.svcCtx.IntentionModel.FindTags(l.ctx)
	if err != nil {
		l.Errorf("查询诉求标签失败: %v", err)
		return nil, common.ErrSystem
	}
	resources, total, err := l.svcCtx.IntentionModel.FindResources(l.ctx, req.Code, req.Page, req.Size)
	if err != nil {
		l.Errorf("查询诉求聚合失败: %v", err)
		return nil, common.ErrSystem
	}
	outTags := make([]types.IntentionTag, 0, len(tags))
	for _, tag := range tags {
		outTags = append(outTags, toIntentionTag(tag))
	}
	out := make([]types.IntentionResource, 0, len(resources))
	for _, item := range resources {
		out = append(out, types.IntentionResource{ResourceType: item.ResourceType, SourceId: item.SourceId, Title: item.Title, Subtitle: item.Subtitle, Price: item.Price, Image: item.Image, OrderTarget: item.OrderTarget, TempleCode: item.TempleCode, ServiceCode: item.ServiceCode, MasterCode: item.MasterCode})
	}
	return &types.CustomerIntentionResp{Tags: outTags, Total: total, List: out, Page: req.Page, Size: req.Size}, nil
}

func ListIntentionTags(ctx context.Context, svcCtx *svc.ServiceContext, includeDisabled bool) (*types.IntentionTagListResp, error) {
	var tags []*model.IntentTag
	var err error
	if includeDisabled {
		tags, err = svcCtx.IntentionModel.FindAllTags(ctx)
	} else {
		tags, err = svcCtx.IntentionModel.FindTags(ctx)
	}
	if err != nil {
		return nil, common.ErrSystem
	}
	list := make([]types.IntentionTag, 0, len(tags))
	for _, tag := range tags {
		list = append(list, toIntentionTag(tag))
	}
	return &types.IntentionTagListResp{List: list}, nil
}

func toIntentionTag(tag *model.IntentTag) types.IntentionTag {
	return types.IntentionTag{Code: tag.Code, Name: tag.Name, Description: tag.Description, Icon: tag.Icon, LandingType: tag.LandingType, LandingValue: tag.LandingValue, ActionTitle: tag.ActionTitle, Sort: tag.Sort, Status: tag.Status}
}

// ===== C端分类树 =====

type CustomerCategoryTreeLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCustomerCategoryTreeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CustomerCategoryTreeLogic {
	return &CustomerCategoryTreeLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CustomerCategoryTreeLogic) Tree(req *types.CustomerCategoryTreeReq) (*types.CustomerCategoryTreeResp, error) {
	list, err := l.svcCtx.ProductCategoryModel.ListAll(l.ctx)
	if err != nil {
		l.Errorf("查询分类列表失败: %v", err)
		return nil, common.ErrSystem
	}
	tree := buildCategoryTree(list)
	return &types.CustomerCategoryTreeResp{List: tree}, nil
}

// ===== helper =====

func toTypesProduct(p *model.Product) types.Product {
	return types.Product{
		Id:                p.Id,
		ProductNo:         p.ProductNo,
		Name:              p.Name,
		CategoryId:        p.CategoryId,
		Description:       p.Description,
		MainImage:         p.MainImage,
		Status:            p.Status,
		Price:             p.Price,
		MarketPrice:       p.MarketPrice,
		Stock:             p.Stock,
		Tags:              p.Tags,
		FreightTemplateId: p.FreightTemplateId,
		CreateTime:        p.CreateTime,
		UpdateTime:        p.UpdateTime,
	}
}

func toTypesSku(s *model.ProductSku) types.ProductSku {
	return types.ProductSku{
		Id:        s.Id,
		ProductId: s.ProductId,
		SpecName:  s.SpecName,
		SpecValue: s.SpecValue,
		Price:     s.Price,
		Stock:     s.Stock,
		SkuNo:     s.SkuNo,
	}
}

func toTypesImage(img *model.ProductImage) types.ProductImage {
	return types.ProductImage{
		Id:        img.Id,
		ProductId: img.ProductId,
		ImageUrl:  img.ImageUrl,
		Sort:      img.Sort,
		Type:      img.Type,
	}
}

func buildCategoryTree(list []*model.ProductCategory) []types.ProductCategory {
	nodeMap := make(map[int64]*types.ProductCategory)
	var roots []types.ProductCategory
	for _, c := range list {
		node := types.ProductCategory{
			Id:       c.Id,
			ParentId: c.ParentId,
			Name:     c.Name,
			Level:    c.Level,
			Sort:     c.Sort,
		}
		nodeMap[c.Id] = &node
	}
	for _, c := range list {
		node := nodeMap[c.Id]
		if c.ParentId == 0 {
			roots = append(roots, *node)
		} else if parent, ok := nodeMap[c.ParentId]; ok {
			parent.Children = append(parent.Children, *node)
		}
	}
	return roots
}
