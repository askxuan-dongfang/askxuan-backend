package logic

import (
	"context"
	"encoding/json"

	"github.com/askxuan/common"
	"github.com/askxuan/diy-service/internal/model"
	"github.com/askxuan/diy-service/internal/svc"
	"github.com/askxuan/diy-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ErrDesignNotFound 设计不存在
var ErrDesignNotFound = common.NewBizError(40414, "设计不存在")

// DesignListLogic 设计广场列表
type DesignListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDesignListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DesignListLogic {
	return &DesignListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DesignListLogic) List(req *types.DesignListReq) (*types.DesignListResp, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Size <= 0 {
		req.Size = 20
	}
	list, total, err := l.svcCtx.DiyDesignModel.FindListPublic(l.ctx, req.Page, req.Size)
	if err != nil {
		l.Errorf("查询设计列表失败: %v", err)
		return nil, common.ErrSystem
	}
	resp := &types.DesignListResp{Total: total, Page: req.Page, Size: req.Size}
	for _, d := range list {
		resp.List = append(resp.List, toTypesDesign(d))
	}
	return resp, nil
}

// DesignSaveLogic 保存设计
type DesignSaveLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDesignSaveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DesignSaveLogic {
	return &DesignSaveLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DesignSaveLogic) Save(req *types.DesignSaveReq) (*types.DesignSaveResp, error) {
	d, err := l.svcCtx.DiyDesignModel.Insert(l.ctx, &model.DiyDesign{
		UserId:           req.UserId,
		Name:             req.Name,
		DesignData:       req.DesignData,
		TotalPrice:       req.TotalPrice,
		Status:           req.Status,
		BlessServiceCode: req.BlessServiceCode,
	})
	if err != nil {
		l.Errorf("保存设计失败: %v", err)
		return nil, common.ErrSystem
	}
	// 缓存设计草稿（24 小时），便于用户下次进入设计页恢复上一次编辑
	if jsonBytes, mErr := json.Marshal(toTypesDesign(d)); mErr == nil {
		_ = l.svcCtx.Redis.Setex("diy:draft:"+req.UserId, string(jsonBytes), 86400)
	}
	return &types.DesignSaveResp{Id: d.Id}, nil
}

// DesignDetailLogic 设计详情
type DesignDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDesignDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DesignDetailLogic {
	return &DesignDetailLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DesignDetailLogic) Detail(req *types.DesignDetailReq) (*types.DiyDesign, error) {
	d, err := l.svcCtx.DiyDesignModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if err == sqlx.ErrNotFound {
			return nil, ErrDesignNotFound
		}
		l.Errorf("查询设计详情失败: %v", err)
		return nil, common.ErrSystem
	}
	t := toTypesDesign(d)
	return &t, nil
}

// MaterialListLogic 材料库列表
type MaterialListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMaterialListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MaterialListLogic {
	return &MaterialListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *MaterialListLogic) List(req *types.MaterialListReq) (*types.MaterialListResp, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Size <= 0 {
		req.Size = 50
	}
	list, total, err := l.svcCtx.MaterialModel.FindList(l.ctx, req.Category, "", req.Page, req.Size)
	if err != nil {
		l.Errorf("查询材料列表失败: %v", err)
		return nil, common.ErrSystem
	}
	resp := &types.MaterialListResp{Total: total, Page: req.Page, Size: req.Size}
	for _, m := range list {
		resp.List = append(resp.List, toTypesMaterial(m))
	}
	return resp, nil
}

// ===== 共享 helpers =====

func toTypesDesign(d *model.DiyDesign) types.DiyDesign {
	return types.DiyDesign{
		Id:               d.Id,
		DesignNo:         d.DesignNo,
		UserId:           d.UserId,
		Name:             d.Name,
		DesignData:       d.DesignData,
		TotalPrice:       d.TotalPrice,
		Status:           d.Status,
		BlessServiceCode: d.BlessServiceCode,
		CreateTime:       d.CreateTime,
	}
}

func toTypesMaterial(m *model.Material) types.Material {
	return types.Material{
		Id:           m.Id,
		Name:         m.Name,
		Spec:         m.Spec,
		UnitPrice:    m.UnitPrice,
		Unit:         m.Unit,
		Category:     m.Category,
		FiveElements: m.FiveElements,
		Image:        m.Image,
		Stock:        m.Stock,
		Status:       m.Status,
	}
}

func toTypesDiyOrderItem(it *model.DiyOrderItem) types.DiyOrderItem {
	return types.DiyOrderItem{
		Id:           it.Id,
		OrderId:      it.OrderId,
		MaterialId:   it.MaterialId,
		SkuId:        it.SkuId,
		MaterialName: it.MaterialName,
		Spec:         it.Spec,
		UnitPrice:    it.UnitPrice,
		Quantity:     it.Quantity,
		Subtype:      it.Subtype,
	}
}

func toTypesBlessingTask(t model.BlessingTask) types.BlessingTask {
	return types.BlessingTask{
		Id:              t.Id,
		TaskNo:          t.TaskNo,
		DiyOrderNo:      t.DiyOrderNo,
		TempleCode:      t.TempleCode,
		MasterCode:      t.MasterCode,
		Status:          t.Status,
		CertificateUrls: t.CertificateUrls,
		AssignTime:      t.AssignTime,
		CompleteTime:    t.CompleteTime,
	}
}

// toTypesDiyOrder 转换为 types.DiyOrder（列表用，不查 items/blessing）
func toTypesDiyOrder(o *model.DiyOrder, items []types.DiyOrderItem, bt model.BlessingTask) types.DiyOrder {
	return types.DiyOrder{
		Id:                  o.Id,
		OrderNo:             o.OrderNo,
		UserId:              o.UserId,
		DesignId:            o.DesignId,
		MaterialFee:         o.MaterialFee,
		BlessFee:            o.BlessFee,
		TotalFee:            o.TotalFee,
		Status:              o.Status,
		PaymentStatus:       o.PaymentStatus,
		AddressId:           o.AddressId,
		Source:              o.Source,
		CreatorId:           o.CreatorId,
		CreatorShareRate:    o.CreatorShareRate,
		OriginalMaterialFee: o.OriginalMaterialFee,
		PriceChanged:        o.PriceChanged == 1,
		DesignSnapshot:      o.DesignSnapshot,
		PricingSnapshot:     o.PricingSnapshot,
		Items:               items,
		BlessingTask:        toTypesBlessingTask(bt),
		CreateTime:          o.CreateTime,
	}
}

// toTypesDiyOrderDetail 查询 items + blessing_task 后转换
func toTypesDiyOrderDetail(ctx context.Context, svcCtx *svc.ServiceContext, o *model.DiyOrder) *types.DiyOrder {
	var items []types.DiyOrderItem
	if ils, err := svcCtx.DiyOrderItemModel.ListByOrderId(ctx, o.Id); err == nil {
		for _, it := range ils {
			items = append(items, toTypesDiyOrderItem(it))
		}
	}
	var bt model.BlessingTask
	if t, err := svcCtx.BlessingTaskModel.FindByDiyOrderNo(ctx, o.OrderNo); err == nil {
		bt = *t
	}
	t := toTypesDiyOrder(o, items, bt)
	return &t
}

// toTypesDiyOrderDetailWithTask 使用预加载的 blessing_task 转换（避免重复查询）
func toTypesDiyOrderDetailWithTask(ctx context.Context, svcCtx *svc.ServiceContext, o *model.DiyOrder, bt model.BlessingTask) *types.DiyOrder {
	var items []types.DiyOrderItem
	if ils, err := svcCtx.DiyOrderItemModel.ListByOrderId(ctx, o.Id); err == nil {
		for _, it := range ils {
			items = append(items, toTypesDiyOrderItem(it))
		}
	}
	t := toTypesDiyOrder(o, items, bt)
	return &t
}
