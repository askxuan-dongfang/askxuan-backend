package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/diy-service/internal/model"
	"github.com/askxuan/diy-service/internal/svc"
	"github.com/askxuan/diy-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// AdminMaterialListLogic 商城台材料列表
type AdminMaterialListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminMaterialListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminMaterialListLogic {
	return &AdminMaterialListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminMaterialListLogic) List(req *types.AdminMaterialListReq) (*types.AdminMaterialListResp, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Size <= 0 {
		req.Size = 20
	}
	list, total, err := l.svcCtx.MaterialModel.FindList(l.ctx, req.Category, req.Keyword, req.Page, req.Size)
	if err != nil {
		l.Errorf("查询材料列表失败: %v", err)
		return nil, common.ErrSystem
	}
	resp := &types.AdminMaterialListResp{Total: total, Page: req.Page, Size: req.Size}
	for _, m := range list {
		resp.List = append(resp.List, toTypesMaterial(m))
	}
	return resp, nil
}

// AdminMaterialCreateLogic 商城台创建材料
type AdminMaterialCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminMaterialCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminMaterialCreateLogic {
	return &AdminMaterialCreateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminMaterialCreateLogic) Create(req *types.AdminMaterialCreateReq) (*types.AdminMaterialCreateResp, error) {
	m, err := l.svcCtx.MaterialModel.Insert(l.ctx, &model.Material{
		Name:         req.Name,
		Spec:         req.Spec,
		UnitPrice:    req.UnitPrice,
		Unit:         req.Unit,
		Category:     req.Category,
		FiveElements: req.FiveElements,
		Image:        req.Image,
		Stock:        req.Stock,
	})
	if err != nil {
		l.Errorf("创建材料失败: %v", err)
		return nil, common.ErrSystem
	}
	return &types.AdminMaterialCreateResp{Id: m.Id}, nil
}

// AdminMaterialUpdateLogic 商城台更新材料
type AdminMaterialUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminMaterialUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminMaterialUpdateLogic {
	return &AdminMaterialUpdateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminMaterialUpdateLogic) Update(req *types.AdminMaterialUpdateReq) (*types.Material, error) {
	err := l.svcCtx.MaterialModel.Update(l.ctx, &model.Material{
		Id:           req.Id,
		Name:         req.Name,
		Spec:         req.Spec,
		UnitPrice:    req.UnitPrice,
		Unit:         req.Unit,
		Category:     req.Category,
		FiveElements: req.FiveElements,
		Image:        req.Image,
		Stock:        req.Stock,
	})
	if err != nil {
		l.Errorf("更新材料失败: %v", err)
		return nil, common.ErrSystem
	}
	m, err := l.svcCtx.MaterialModel.FindOne(l.ctx, req.Id)
	if err != nil {
		return nil, common.ErrSystem
	}
	t := toTypesMaterial(m)
	return &t, nil
}

// AdminMaterialStatusLogic 商城台材料上下架
type AdminMaterialStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminMaterialStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminMaterialStatusLogic {
	return &AdminMaterialStatusLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminMaterialStatusLogic) Status(req *types.AdminMaterialStatusReq) (*types.Material, error) {
	if req.Status != model.MaterialStatusOnShelf && req.Status != model.MaterialStatusOffShelf {
		return nil, common.ErrParam
	}
	err := l.svcCtx.MaterialModel.UpdateStatus(l.ctx, req.Id, req.Status)
	if err != nil {
		l.Errorf("更新材料状态失败: %v", err)
		return nil, common.ErrSystem
	}
	m, err := l.svcCtx.MaterialModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if err == sqlx.ErrNotFound {
			return nil, common.ErrParam
		}
		return nil, common.ErrSystem
	}
	t := toTypesMaterial(m)
	return &t, nil
}

// AdminBlessingServiceListLogic 商城台加持服务列表
type AdminBlessingServiceListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminBlessingServiceListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminBlessingServiceListLogic {
	return &AdminBlessingServiceListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminBlessingServiceListLogic) List(req *types.AdminBlessingServiceListReq) (*types.AdminBlessingServiceListResp, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Size <= 0 {
		req.Size = 20
	}
	list, total, err := l.svcCtx.ExtraServiceModel.FindList(l.ctx, req.Page, req.Size)
	if err != nil {
		l.Errorf("查询加持服务失败: %v", err)
		return nil, common.ErrSystem
	}
	resp := &types.AdminBlessingServiceListResp{Page: req.Page, Size: req.Size, Total: total}
	for _, s := range list {
		resp.List = append(resp.List, toTypesBlessingService(s))
	}
	return resp, nil
}

// AdminBlessingServiceCreateLogic 商城台创建加持服务
type AdminBlessingServiceCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminBlessingServiceCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminBlessingServiceCreateLogic {
	return &AdminBlessingServiceCreateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminBlessingServiceCreateLogic) Create(req *types.AdminBlessingServiceCreateReq) (*types.AdminBlessingServiceCreateResp, error) {
	if req.ServiceName == "" || req.TempleCode == "" {
		return nil, common.ErrParam
	}
	if req.Status != "" && req.Status != model.BlessingServiceStatusOnShelf && req.Status != model.BlessingServiceStatusOffShelf {
		return nil, common.ErrParamInvalid
	}
	s, err := l.svcCtx.ExtraServiceModel.Insert(l.ctx, &model.ExtraService{
		Name:        req.ServiceName,
		TempleCode:  req.TempleCode,
		MasterCode:  req.MasterCode,
		Price:       req.Price,
		Description: req.Description,
		Status:      req.Status,
	})
	if err != nil {
		l.Errorf("创建加持服务失败: %v", err)
		return nil, common.ErrSystem
	}
	return &types.AdminBlessingServiceCreateResp{Id: s.Id}, nil
}

// AdminBlessingServiceUpdateLogic 商城台更新加持服务
type AdminBlessingServiceUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminBlessingServiceUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminBlessingServiceUpdateLogic {
	return &AdminBlessingServiceUpdateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminBlessingServiceUpdateLogic) Update(req *types.AdminBlessingServiceUpdateReq) (*types.BlessingService, error) {
	if req.ServiceName == "" || req.TempleCode == "" {
		return nil, common.ErrParam
	}
	if req.Status != model.BlessingServiceStatusOnShelf && req.Status != model.BlessingServiceStatusOffShelf {
		return nil, common.ErrParamInvalid
	}
	existing, err := l.svcCtx.ExtraServiceModel.FindOne(l.ctx, req.Id)
	if err == sqlx.ErrNotFound {
		return nil, common.NewBizError(40415, "加持服务不存在")
	}
	if err != nil {
		l.Errorf("查询加持服务失败: %v", err)
		return nil, common.ErrSystem
	}
	existing.Name = req.ServiceName
	existing.TempleCode = req.TempleCode
	existing.MasterCode = req.MasterCode
	existing.Price = req.Price
	existing.Description = req.Description
	existing.Status = req.Status
	if err := l.svcCtx.ExtraServiceModel.Update(l.ctx, existing); err != nil {
		l.Errorf("更新加持服务失败: %v", err)
		return nil, common.ErrSystem
	}
	t := toTypesBlessingService(existing)
	return &t, nil
}

// AdminBlessingServiceDeleteLogic 商城台删除加持服务
type AdminBlessingServiceDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminBlessingServiceDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminBlessingServiceDeleteLogic {
	return &AdminBlessingServiceDeleteLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminBlessingServiceDeleteLogic) Delete(req *types.AdminBlessingServiceDeleteReq) error {
	deleted, err := l.svcCtx.ExtraServiceModel.Delete(l.ctx, req.Id)
	if err != nil {
		l.Errorf("删除加持服务失败: %v", err)
		return common.ErrSystem
	}
	if !deleted {
		return common.NewBizError(40415, "加持服务不存在")
	}
	return nil
}

func toTypesBlessingService(s *model.ExtraService) types.BlessingService {
	return types.BlessingService{
		Id:          s.Id,
		ServiceCode: s.Code,
		ServiceName: s.Name,
		TempleCode:  s.TempleCode,
		MasterCode:  s.MasterCode,
		Price:       s.Price,
		Description: s.Description,
		Status:      s.Status,
	}
}
