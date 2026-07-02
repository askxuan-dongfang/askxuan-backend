package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/askxuan/common"
	"github.com/askxuan/order-service/internal/model"
	"github.com/askxuan/order-service/internal/svc"
	"github.com/askxuan/order-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// AdminOrderListLogic 商城台订单列表
type AdminOrderListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminOrderListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminOrderListLogic {
	return &AdminOrderListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminOrderListLogic) List(req *types.AdminOrderListReq) (*types.AdminOrderListResp, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Size <= 0 {
		req.Size = 20
	}
	list, total, err := l.svcCtx.ShopOrderModel.FindListAdmin(l.ctx, req.Status, req.Page, req.Size)
	if err != nil {
		l.Errorf("管理台查询订单列表失败: %v", err)
		return nil, common.ErrSystem
	}
	resp := &types.AdminOrderListResp{Total: total, Page: req.Page, Size: req.Size}
	for _, o := range list {
		resp.List = append(resp.List, toTypesOrder(o, nil, model.ShopOrderLogistics{}))
	}
	return resp, nil
}

// AdminOrderDetailLogic 商城台订单详情
type AdminOrderDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminOrderDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminOrderDetailLogic {
	return &AdminOrderDetailLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminOrderDetailLogic) Detail(req *types.AdminOrderDetailReq) (*types.ShopOrder, error) {
	o, err := l.svcCtx.ShopOrderModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if err == sqlx.ErrNotFound {
			return nil, common.ErrOrderNotFound
		}
		return nil, common.ErrSystem
	}
	return toTypesOrderDetail(l.ctx, l.svcCtx, o), nil
}

// AdminOrderShipLogic 商城台发货
type AdminOrderShipLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminOrderShipLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminOrderShipLogic {
	return &AdminOrderShipLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminOrderShipLogic) Ship(req *types.AdminOrderShipReq) (*types.ShopOrder, error) {
	o, err := l.svcCtx.ShopOrderModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if err == sqlx.ErrNotFound {
			return nil, common.ErrOrderNotFound
		}
		return nil, common.ErrSystem
	}
	if !model.CanOrderTransit(o.Status, model.OrderStatusShipped) {
		return nil, common.ErrStatusInvalid
	}
	// 写物流记录
	_, err = l.svcCtx.ShopOrderLogisticsModel.Insert(l.ctx, &model.ShopOrderLogistics{
		OrderId:        o.Id,
		ExpressCompany: req.ExpressCompany,
		TrackingNo:     req.TrackingNo,
	})
	if err != nil {
		l.Errorf("写入物流记录失败: %v", err)
		return nil, common.ErrSystem
	}
	// 更新订单状态
	updated, err := l.svcCtx.ShopOrderModel.UpdateStatus(l.ctx, req.Id, model.OrderStatusShipped)
	if err != nil {
		return nil, common.ErrSystem
	}
	// 发货后失效订单状态缓存
	_, _ = l.svcCtx.Redis.Del("order:status:" + updated.OrderNo)
	// 发 MQ 通知发货（order.events action=shipped）
	_ = l.svcCtx.MqProducer.Publish(l.ctx, mqOrderNotify(updated.OrderNo, updated.UserId, "shipped"))
	return toTypesOrderDetail(l.ctx, l.svcCtx, updated), nil
}

// AdminReturnListLogic 退换货列表
type AdminReturnListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminReturnListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminReturnListLogic {
	return &AdminReturnListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminReturnListLogic) List(req *types.AdminReturnListReq) (*types.AdminReturnListResp, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Size <= 0 {
		req.Size = 20
	}
	list, total, err := l.svcCtx.ReturnOrderModel.FindList(l.ctx, req.Status, req.Page, req.Size)
	if err != nil {
		l.Errorf("查询退换货列表失败: %v", err)
		return nil, common.ErrSystem
	}
	resp := &types.AdminReturnListResp{Total: total, Page: req.Page, Size: req.Size}
	for _, r := range list {
		resp.List = append(resp.List, toTypesReturn(r))
	}
	return resp, nil
}

// AdminReturnReviewLogic 审核退换货
type AdminReturnReviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminReturnReviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminReturnReviewLogic {
	return &AdminReturnReviewLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminReturnReviewLogic) Review(req *types.AdminReturnReviewReq) (*types.ReturnOrder, error) {
	r, err := l.svcCtx.ReturnOrderModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if err == sqlx.ErrNotFound {
			return nil, common.ErrOrderNotFound
		}
		return nil, common.ErrSystem
	}
	var targetStatus string
	switch req.Action {
	case "approve":
		targetStatus = model.ReturnStatusApproved
	case "reject":
		targetStatus = model.ReturnStatusRejected
	default:
		return nil, common.ErrParam
	}
	if !model.CanReturnTransit(r.Status, targetStatus) {
		return nil, common.ErrStatusInvalid
	}
	updated, err := l.svcCtx.ReturnOrderModel.UpdateStatus(l.ctx, req.Id, targetStatus)
	if err != nil {
		return nil, common.ErrSystem
	}
	t := toTypesReturn(updated)
	return &t, nil
}

// AdminReturnRefundLogic 退款
type AdminReturnRefundLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminReturnRefundLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminReturnRefundLogic {
	return &AdminReturnRefundLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminReturnRefundLogic) Refund(req *types.AdminReturnRefundReq) (*types.ReturnOrder, error) {
	r, err := l.svcCtx.ReturnOrderModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if err == sqlx.ErrNotFound {
			return nil, common.ErrOrderNotFound
		}
		return nil, common.ErrSystem
	}
	if !model.CanReturnTransit(r.Status, model.ReturnStatusRefunding) {
		return nil, common.ErrStatusInvalid
	}
	// 先流转到 refunding
	_, err = l.svcCtx.ReturnOrderModel.UpdateStatus(l.ctx, req.Id, model.ReturnStatusRefunding)
	if err != nil {
		return nil, common.ErrSystem
	}
	// 调 payment-service 退款接口（HTTP）
	if err := callPaymentRefund(l.ctx, l.svcCtx, r, req.Amount); err != nil {
		l.Errorf("调用支付退款失败: %v", err)
		// 退款失败不改变本地状态，由人工处理
	}
	// 流转到 completed
	updated, err := l.svcCtx.ReturnOrderModel.UpdateStatus(l.ctx, req.Id, model.ReturnStatusCompleted)
	if err != nil {
		return nil, common.ErrSystem
	}
	t := toTypesReturn(updated)
	return &t, nil
}

// callPaymentRefund 调用 payment-service 退款接口
func callPaymentRefund(ctx context.Context, svcCtx *svc.ServiceContext, r *model.ReturnOrder, amount float64) error {
	payload := map[string]interface{}{
		"paymentNo": fmt.Sprintf("PAY-%d", r.OrderId), // 临时构造，实际应查询订单关联的支付单号
		"amount":    amount,
		"reason":    r.Reason,
	}
	body, _ := json.Marshal(payload)
	// payment-service 端口 8090；直连调用
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://localhost:8090/api/v1/payments/refund", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("退款接口返回 %d", resp.StatusCode)
	}
	return nil
}

func toTypesReturn(r *model.ReturnOrder) types.ReturnOrder {
	return types.ReturnOrder{
		Id:           r.Id,
		ReturnNo:     r.ReturnNo,
		OrderId:      r.OrderId,
		Type:         r.Type,
		Reason:       r.Reason,
		Status:       r.Status,
		RefundAmount: r.RefundAmount,
		CreateTime:   r.CreateTime,
	}
}
