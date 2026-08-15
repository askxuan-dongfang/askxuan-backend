package logic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/askxuan/common"
	"github.com/askxuan/order-service/internal/model"
	"github.com/askxuan/order-service/internal/mq"
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

// OrderReportLogic 商城经营报表
type OrderReportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewOrderReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OrderReportLogic {
	return &OrderReportLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *OrderReportLogic) Report() (*types.OrderReportResp, error) {
	stats, err := l.svcCtx.ShopOrderModel.GetReportStats(l.ctx)
	if err != nil {
		l.Errorf("查询商城报表统计失败: %v", err)
		return nil, common.ErrSystem
	}
	trendRows, err := l.svcCtx.ShopOrderModel.GetReportTrend(l.ctx, 7)
	if err != nil {
		l.Errorf("查询商城趋势失败: %v", err)
		return nil, common.ErrSystem
	}
	topRows, err := l.svcCtx.ShopOrderModel.GetReportTopProducts(l.ctx, 5)
	if err != nil {
		l.Errorf("查询热销商品失败: %v", err)
		return nil, common.ErrSystem
	}
	resp := &types.OrderReportResp{
		TodayOrders: stats.TodayOrders,
		TodaySales:  stats.TodaySales,
		PendingShip: stats.PendingShip,
		TotalOrders: stats.TotalOrders,
		TotalSales:  stats.TotalSales,
		Trend:       make([]types.OrderReportTrendPoint, 0, len(trendRows)),
		TopProducts: make([]types.OrderReportTopProduct, 0, len(topRows)),
	}
	for _, r := range trendRows {
		resp.Trend = append(resp.Trend, types.OrderReportTrendPoint{Date: r.Date, Sales: r.Sales, Orders: r.Orders})
	}
	for _, t := range topRows {
		resp.TopProducts = append(resp.TopProducts, types.OrderReportTopProduct{
			ProductId: t.ProductId, ProductName: t.ProductName, Sales: t.Sales, OrderCount: t.OrderCount,
		})
	}
	return resp, nil
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

// AdminReturnDetailLogic 退换货详情
type AdminReturnDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminReturnDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminReturnDetailLogic {
	return &AdminReturnDetailLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminReturnDetailLogic) Detail(req *types.AdminReturnDetailReq) (*types.ReturnOrder, error) {
	r, err := l.svcCtx.ReturnOrderModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if err == sqlx.ErrNotFound {
			return nil, common.ErrOrderNotFound
		}
		return nil, common.ErrSystem
	}
	t := toTypesReturn(r)
	return &t, nil
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

	// 事务：状态流转 refunding + 写入 outbox 消息（保证原子性）
	// 退款实际由 payment-service 异步处理，order-service 通过 outbox 可靠投递退款请求，
	// 通过消费 payment.refund.completed 事件将退货单流转到 completed。
	payload := mq.BuildRefundRequestPayload(r, req.Amount)
	err = l.svcCtx.DB.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		if err := l.svcCtx.ReturnOrderModel.UpdateStatusWithSession(ctx, session, req.Id, model.ReturnStatusRefunding); err != nil {
			return err
		}
		if err := model.InsertOutbox(ctx, session, r.ReturnNo, mq.MessageTypeRefundRequest, payload); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		l.Errorf("退款事务失败（refunding + outbox）: %v", err)
		return nil, common.ErrSystem
	}
	l.Infof("退款已异步提交，returnNo=%s amount=%.2f，等待 payment.refund.completed 事件", r.ReturnNo, req.Amount)

	// 重新查询返回最新状态（refunding）
	updated, err := l.svcCtx.ReturnOrderModel.FindOne(l.ctx, req.Id)
	if err != nil {
		return nil, common.ErrSystem
	}
	t := toTypesReturn(updated)
	return &t, nil
}

// callPaymentRefund 同步调用 payment-service 退款接口（DEPRECATED）
// 阶段 5 已改为 Outbox + 异步 MQ 模式：Refund 函数不再调用本函数。
// 保留代码作为 fallback 与历史参考，后续 payment-service 完全切换到 outbox 消费后可移除。
//
// 通过网关调用（网关路由到 etcd 发现的实例），并携带内部服务 JWT 绕过管理台角色校验
func callPaymentRefund(ctx context.Context, svcCtx *svc.ServiceContext, r *model.ReturnOrder, amount float64) error {
	payload := map[string]interface{}{
		"paymentNo": fmt.Sprintf("PAY-%d", r.OrderId), // 临时构造，实际应查询订单关联的支付单号
		"amount":    amount,
		"reason":    r.Reason,
	}
	body, _ := json.Marshal(payload)

	// 通过网关调用 payment-service（网关路由到 etcd 发现的实例）
	refundURL := "http://localhost:8080/api/v1/payments/refund"
	if svcCtx.Config.PaymentGateway != "" {
		refundURL = svcCtx.Config.PaymentGateway + "/api/v1/payments/refund"
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, refundURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("构造退款请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 构造内部服务 JWT（service 角色，绕过管理台角色校验）
	token, err := common.SignServiceToken(svcCtx.Config.AuthSecret, "order-service")
	if err != nil {
		return fmt.Errorf("签名内部 token 失败: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("调用退款接口失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("退款接口返回 %d: %s", resp.StatusCode, string(respBody))
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
