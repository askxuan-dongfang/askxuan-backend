package logic

import (
	"context"
	"time"

	"github.com/askxuan/common"
	"github.com/askxuan/order-service/internal/model"
	"github.com/askxuan/order-service/internal/mq"
	"github.com/askxuan/order-service/internal/svc"
	"github.com/askxuan/order-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// OrderCreateLogic 创建订单
type OrderCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOrderCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OrderCreateLogic {
	return &OrderCreateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *OrderCreateLogic) Create(req *types.OrderCreateReq) (*types.OrderCreateResp, error) {
	if len(req.Items) == 0 {
		return nil, common.ErrParam
	}

	// 计算总金额
	var total float64
	for _, it := range req.Items {
		total += it.Price * float64(it.Quantity)
	}

	// 事务写 order + items
	var orderNo string
	var orderId int64
	err := l.svcCtx.DB.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		o, err := l.svcCtx.ShopOrderModel.Insert(l.ctx, &model.ShopOrder{
			UserId:      req.UserId,
			TotalAmount: total,
			PayAmount:   total,
			Status:      model.OrderStatusPendingPayment,
			AddressId:   req.AddressId,
			Note:        req.Note,
		})
		if err != nil {
			return err
		}
		orderNo = o.OrderNo
		orderId = o.Id
		for _, it := range req.Items {
			_, err := l.svcCtx.ShopOrderItemModel.Insert(l.ctx, &model.ShopOrderItem{
				OrderId:     o.Id,
				ProductId:   it.ProductId,
				SkuId:       it.SkuId,
				ProductName: it.ProductName,
				SkuSpec:     it.SkuSpec,
				Price:       it.Price,
				Quantity:    it.Quantity,
				Image:       it.Image,
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		l.Errorf("创建订单失败: %v", err)
		return nil, common.ErrSystem
	}

	// 发 MQ 通知订单创建（失败不阻塞主流程）
	_ = l.svcCtx.MqProducer.Publish(l.ctx, mqOrderNotify(orderNo, req.UserId, "created"))

	return &types.OrderCreateResp{Id: orderId, OrderNo: orderNo}, nil
}

// OrderListLogic 我的订单列表
type OrderListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOrderListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OrderListLogic {
	return &OrderListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *OrderListLogic) List(req *types.OrderListReq) (*types.OrderListResp, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Size <= 0 {
		req.Size = 20
	}
	list, total, err := l.svcCtx.ShopOrderModel.FindListByUser(l.ctx, req.UserId, req.Status, req.Page, req.Size)
	if err != nil {
		l.Errorf("查询订单列表失败: %v", err)
		return nil, common.ErrSystem
	}
	resp := &types.OrderListResp{Total: total, Page: req.Page, Size: req.Size}
	for _, o := range list {
		resp.List = append(resp.List, toTypesOrder(o, nil, model.ShopOrderLogistics{}))
	}
	return resp, nil
}

// OrderDetailLogic 订单详情
type OrderDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOrderDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OrderDetailLogic {
	return &OrderDetailLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *OrderDetailLogic) Detail(req *types.OrderDetailReq) (*types.ShopOrder, error) {
	o, err := l.svcCtx.ShopOrderModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if err == sqlx.ErrNotFound {
			return nil, common.ErrOrderNotFound
		}
		l.Errorf("查询订单详情失败: %v", err)
		return nil, common.ErrSystem
	}
	// 缓存订单状态（30 秒），供仅需要状态的场景读取
	_ = l.svcCtx.Redis.Setex("order:status:"+o.OrderNo, o.Status, 30)
	return toTypesOrderDetail(l.ctx, l.svcCtx, o), nil
}

// OrderConfirmLogic 确认收货
type OrderConfirmLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOrderConfirmLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OrderConfirmLogic {
	return &OrderConfirmLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *OrderConfirmLogic) Confirm(req *types.OrderConfirmReq) (*types.ShopOrder, error) {
	o, err := l.svcCtx.ShopOrderModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if err == sqlx.ErrNotFound {
			return nil, common.ErrOrderNotFound
		}
		return nil, common.ErrSystem
	}
	if !model.CanOrderTransit(o.Status, model.OrderStatusCompleted) {
		return nil, common.ErrStatusInvalid
	}
	updated, err := l.svcCtx.ShopOrderModel.UpdateStatus(l.ctx, req.Id, model.OrderStatusCompleted)
	if err != nil {
		return nil, common.ErrSystem
	}
	// 状态变更后失效订单状态缓存
	_, _ = l.svcCtx.Redis.Del("order:status:" + updated.OrderNo)
	_ = l.svcCtx.MqProducer.Publish(l.ctx, mqOrderNotify(updated.OrderNo, updated.UserId, "completed"))
	return toTypesOrderDetail(l.ctx, l.svcCtx, updated), nil
}

// OrderReturnLogic 申请退换货
type OrderReturnLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOrderReturnLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OrderReturnLogic {
	return &OrderReturnLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *OrderReturnLogic) Return(req *types.OrderReturnReq) (*types.OrderReturnResp, error) {
	o, err := l.svcCtx.ShopOrderModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if err == sqlx.ErrNotFound {
			return nil, common.ErrOrderNotFound
		}
		return nil, common.ErrSystem
	}
	r, err := l.svcCtx.ReturnOrderModel.Insert(l.ctx, &model.ReturnOrder{
		OrderId:      o.Id,
		Type:         req.Type,
		Reason:       req.Reason,
		RefundAmount: o.PayAmount,
	})
	if err != nil {
		l.Errorf("创建退换货失败: %v", err)
		return nil, common.ErrSystem
	}
	_ = l.svcCtx.MqProducer.Publish(l.ctx, mqOrderNotify(o.OrderNo, o.UserId, "return"))
	return &types.OrderReturnResp{Id: r.Id, ReturnNo: r.ReturnNo}, nil
}

// ===== helpers =====

func mqOrderNotify(orderNo, userId, action string) mq.OrderNotify {
	return mq.OrderNotify{OrderId: orderNo, UserId: userId, Action: action, Time: time.Now().Format("2006-01-02 15:04:05")}
}

// toTypesOrder 转换为 types.ShopOrder（列表用，不查 items/logistics）
func toTypesOrder(o *model.ShopOrder, items []types.ShopOrderItem, logistics model.ShopOrderLogistics) types.ShopOrder {
	return types.ShopOrder{
		Id:          o.Id,
		OrderNo:     o.OrderNo,
		UserId:      o.UserId,
		TotalAmount: o.TotalAmount,
		PayAmount:   o.PayAmount,
		Status:      o.Status,
		AddressId:   o.AddressId,
		Note:        o.Note,
		Items:       items,
		Logistics:   toTypesLogistics(logistics),
		CreateTime:  o.CreateTime,
	}
}

// toTypesOrderDetail 查询 items + logistics 后转换
func toTypesOrderDetail(ctx context.Context, svcCtx *svc.ServiceContext, o *model.ShopOrder) *types.ShopOrder {
	var items []types.ShopOrderItem
	if ils, err := svcCtx.ShopOrderItemModel.ListByOrderId(ctx, o.Id); err == nil {
		for _, it := range ils {
			items = append(items, toTypesItem(it))
		}
	}
	var logistics model.ShopOrderLogistics
	if lg, err := svcCtx.ShopOrderLogisticsModel.FindByOrderId(ctx, o.Id); err == nil {
		logistics = *lg
	}
	t := toTypesOrder(o, items, logistics)
	return &t
}

func toTypesItem(it *model.ShopOrderItem) types.ShopOrderItem {
	return types.ShopOrderItem{
		Id:          it.Id,
		OrderId:     it.OrderId,
		ProductId:   it.ProductId,
		SkuId:       it.SkuId,
		ProductName: it.ProductName,
		SkuSpec:     it.SkuSpec,
		Price:       it.Price,
		Quantity:    it.Quantity,
		Image:       it.Image,
	}
}

func toTypesLogistics(l model.ShopOrderLogistics) types.ShopOrderLogistics {
	return types.ShopOrderLogistics{
		Id:             l.Id,
		OrderId:        l.OrderId,
		ExpressCompany: l.ExpressCompany,
		TrackingNo:     l.TrackingNo,
		ShipTime:       l.ShipTime,
	}
}
