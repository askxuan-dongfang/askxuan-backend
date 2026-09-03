package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/askxuan/common"
	"github.com/askxuan/diy-service/internal/model"
	"github.com/askxuan/diy-service/internal/svc"
	"github.com/askxuan/diy-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// DiyOrderCreateLogic 创建DIY订单
type DiyOrderCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

type DiyOrderAvailabilityLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDiyOrderAvailabilityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DiyOrderAvailabilityLogic {
	return &DiyOrderAvailabilityLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DiyOrderAvailabilityLogic) Check(req *types.DiyOrderAvailabilityReq) (*types.DiyOrderAvailabilityResp, error) {
	items := req.Items
	if len(items) == 0 {
		if req.DesignId <= 0 {
			return nil, common.ErrParam
		}
		design, err := l.svcCtx.DiyDesignModel.FindOne(l.ctx, req.DesignId)
		if err != nil {
			if errors.Is(err, sqlx.ErrNotFound) {
				return nil, ErrDesignNotFound
			}
			return nil, common.ErrSystem
		}
		items, err = parseDesignOrderItems(design.DesignData)
		if err != nil || len(items) == 0 {
			return nil, common.ErrParam
		}
	}
	availability, err := model.CheckPricedOrderItems(l.ctx, l.svcCtx.DB, toPricedInputs(items))
	if err != nil {
		return nil, mapPricingError(err)
	}
	resp := &types.DiyOrderAvailabilityResp{
		Orderable: availability.Orderable, MaterialFee: availability.MaterialFee,
		OriginalMaterialFee: availability.OriginalMaterialFee, PriceChanged: availability.PriceChanged,
		Issues: make([]types.DiyOrderAvailabilityIssue, 0, len(availability.Issues)),
	}
	for _, issue := range availability.Issues {
		resp.Issues = append(resp.Issues, types.DiyOrderAvailabilityIssue{
			MaterialId: issue.MaterialId, MaterialName: issue.MaterialName, Spec: issue.Spec,
			Quantity: issue.Quantity, Reason: issue.Reason, Message: availabilityIssueMessage(issue),
		})
	}
	return resp, nil
}

func NewDiyOrderCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DiyOrderCreateLogic {
	return &DiyOrderCreateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DiyOrderCreateLogic) Create(req *types.DiyOrderCreateReq) (*types.DiyOrderCreateResp, error) {
	if req.UserId == "" || req.DesignId == 0 || req.AddressId == 0 || len(req.Items) == 0 {
		return nil, common.ErrParam
	}
	design, err := l.svcCtx.DiyDesignModel.FindOne(l.ctx, req.DesignId)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, ErrDesignNotFound
		}
		return nil, common.ErrSystem
	}
	if design.UserId != req.UserId {
		return nil, common.ErrForbidden
	}
	result, err := model.CreatePricedOrder(l.ctx, l.svcCtx.DB, l.svcCtx.DiyOrderModel, l.svcCtx.DiyOrderItemModel, model.PricedOrderInput{
		UserId: req.UserId, Design: design, Items: toPricedInputs(req.Items), BlessServiceCode: req.BlessServiceCode,
		AddressId: req.AddressId, Source: "custom",
	})
	if err != nil {
		return nil, mapPricingError(err)
	}
	return toOrderCreateResp(result), nil
}

// DiyDesignOrderCreateLogic 从设计广场作品直接创建DIY订单
type DiyDesignOrderCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDiyDesignOrderCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DiyDesignOrderCreateLogic {
	return &DiyDesignOrderCreateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DiyDesignOrderCreateLogic) Create(req *types.DiyDesignOrderCreateReq) (*types.DiyOrderCreateResp, error) {
	if req.Id == 0 || req.UserId == "" || req.AddressId == 0 {
		return nil, common.ErrParam
	}

	design, err := l.svcCtx.DiyDesignModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if err == sqlx.ErrNotFound {
			return nil, ErrDesignNotFound
		}
		l.Errorf("查询DIY设计失败: %v", err)
		return nil, common.ErrSystem
	}
	if design.Status != model.DesignStatusPublic && design.Status != model.DesignStatusApproved {
		return nil, common.NewBizError(40907, "作品尚未上架，无法下单")
	}

	items, err := parseDesignOrderItems(design.DesignData)
	if err != nil || len(items) == 0 {
		l.Errorf("解析DIY设计材料失败: designId=%d err=%v", design.Id, err)
		return nil, common.ErrParam
	}

	blessCode := req.BlessServiceCode
	if blessCode == "" {
		blessCode = design.BlessServiceCode
	}
	result, err := model.CreatePricedOrder(l.ctx, l.svcCtx.DB, l.svcCtx.DiyOrderModel, l.svcCtx.DiyOrderItemModel, model.PricedOrderInput{
		UserId: req.UserId, Design: design, Items: toPricedInputs(items), BlessServiceCode: blessCode,
		AddressId: req.AddressId, Source: "design_square", CreatorId: design.UserId,
	})
	if err != nil {
		l.Errorf("设计广场下单失败 designId=%d: %v", design.Id, err)
		return nil, mapPricingError(err)
	}
	return toOrderCreateResp(result), nil
}

func toPricedInputs(items []types.DiyOrderItem) []model.PricedOrderItemInput {
	result := make([]model.PricedOrderItemInput, 0, len(items))
	for _, item := range items {
		result = append(result, model.PricedOrderItemInput{
			MaterialId: item.MaterialId, Spec: item.Spec, Quantity: item.Quantity,
			Subtype: item.Subtype, SnapshotUnitPrice: item.UnitPrice,
		})
	}
	return result
}

func mapPricingError(err error) error {
	switch {
	case errors.Is(err, model.ErrOrderMaterialUnavailable):
		var unavailable *model.OrderMaterialUnavailableError
		if errors.As(err, &unavailable) && unavailable.MaterialName != "" {
			return common.NewBizError(40907, fmt.Sprintf("材料「%s」已下架或规格 %s 不可用，请重新选择", unavailable.MaterialName, unavailable.Spec))
		}
		return common.NewBizError(40907, "材料已下架或规格不可用，请重新选择")
	case errors.Is(err, model.ErrOrderStockInsufficient):
		return common.ErrStockInsufficient
	case errors.Is(err, model.ErrOrderBlessingUnavailable):
		return common.NewBizError(40908, "加持服务已不可用，请重新选择")
	case errors.Is(err, model.ErrOrderPricingInvalid):
		return common.ErrParamInvalid
	default:
		return common.ErrSystem
	}
}

func availabilityIssueMessage(issue model.PricedOrderAvailabilityIssue) string {
	switch issue.Reason {
	case "stock_insufficient":
		return fmt.Sprintf("%s %s 库存不足", issue.MaterialName, issue.Spec)
	case "spec_unavailable":
		return fmt.Sprintf("%s 的 %s 规格已不可用", issue.MaterialName, issue.Spec)
	case "off_shelf":
		return fmt.Sprintf("%s 已下架", issue.MaterialName)
	default:
		return fmt.Sprintf("%s 已不可用", issue.MaterialName)
	}
}

func toOrderCreateResp(result *model.PricedOrderResult) *types.DiyOrderCreateResp {
	order := result.Order
	items := make([]types.DiyOrderItem, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, types.DiyOrderItem{
			Id: item.Id, OrderId: item.OrderId, MaterialId: item.MaterialId, SkuId: item.SkuId,
			MaterialName: item.MaterialName, Spec: item.Spec, UnitPrice: item.UnitPrice,
			Quantity: item.Quantity, Subtype: item.Subtype,
		})
	}
	return &types.DiyOrderCreateResp{
		Id: order.Id, OrderNo: order.OrderNo, UserId: order.UserId, DesignId: order.DesignId,
		MaterialFee: order.MaterialFee, BlessFee: order.BlessFee, TotalFee: order.TotalFee,
		Status: order.Status, PaymentStatus: order.PaymentStatus, AddressId: order.AddressId, Items: items, Source: order.Source,
		CreatorId: order.CreatorId, CreatorShareRate: order.CreatorShareRate,
		OriginalMaterialFee: order.OriginalMaterialFee, PriceChanged: order.PriceChanged == 1,
		DesignSnapshot: order.DesignSnapshot, PricingSnapshot: order.PricingSnapshot, CreateTime: order.CreateTime,
	}
}

func parseDesignOrderItems(raw string) ([]types.DiyOrderItem, error) {
	if raw == "" {
		return nil, common.ErrParam
	}
	var direct []types.DiyOrderItem
	if err := json.Unmarshal([]byte(raw), &direct); err == nil && len(direct) > 0 {
		return direct, nil
	}

	var wrapped struct {
		Items     []types.DiyOrderItem `json:"items"`
		Materials []types.DiyOrderItem `json:"materials"`
	}
	if err := json.Unmarshal([]byte(raw), &wrapped); err != nil {
		return nil, err
	}
	if len(wrapped.Items) > 0 {
		return wrapped.Items, nil
	}
	return wrapped.Materials, nil
}

// DiyOrderListLogic 我的DIY订单列表
type DiyOrderListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDiyOrderListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DiyOrderListLogic {
	return &DiyOrderListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DiyOrderListLogic) List(req *types.DiyOrderListReq) (*types.DiyOrderListResp, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Size <= 0 {
		req.Size = 20
	}
	list, total, err := l.svcCtx.DiyOrderModel.FindListByUser(l.ctx, req.UserId, req.Status, req.Page, req.Size)
	if err != nil {
		l.Errorf("查询DIY订单列表失败: %v", err)
		return nil, common.ErrSystem
	}
	resp := &types.DiyOrderListResp{Total: total, Page: req.Page, Size: req.Size}
	for _, o := range list {
		resp.List = append(resp.List, toTypesDiyOrder(o, nil, model.BlessingTask{}))
	}
	return resp, nil
}

// DiyOrderDetailLogic DIY订单详情
type DiyOrderDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDiyOrderDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DiyOrderDetailLogic {
	return &DiyOrderDetailLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DiyOrderDetailLogic) Detail(req *types.DiyOrderDetailReq) (*types.DiyOrder, error) {
	o, err := l.svcCtx.DiyOrderModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if err == sqlx.ErrNotFound {
			return nil, common.ErrDiyOrderNotFound
		}
		l.Errorf("查询DIY订单详情失败: %v", err)
		return nil, common.ErrSystem
	}
	return toTypesDiyOrderDetail(l.ctx, l.svcCtx, o), nil
}
