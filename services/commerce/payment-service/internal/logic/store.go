package logic

import (
	"context"
	"encoding/json"

	"github.com/askxuan/common"
	"github.com/askxuan/payment-service/internal/model"
	"github.com/askxuan/payment-service/internal/mq"
	"github.com/askxuan/payment-service/internal/svc"
	"github.com/askxuan/payment-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// ==================== 共享 helper ====================

// isValidChannel 校验支付渠道
func isValidChannel(c string) bool {
	return c == model.PaymentChannelWechat || c == model.PaymentChannelAlipay
}

// isValidOrderType 校验订单类型
func isValidOrderType(t string) bool {
	return t == model.OrderTypeBooking || t == model.OrderTypeShopOrder || t == model.OrderTypeDiyOrder
}

// toTypesPayment 将 model.Payment 转为 types.Payment
func toTypesPayment(p model.Payment) types.Payment {
	return types.Payment{
		Id:         p.Id,
		PaymentNo:  p.PaymentNo,
		OrderType:  p.OrderType,
		OrderNo:    p.OrderNo,
		Amount:     p.Amount,
		Channel:    p.Channel,
		Status:     p.Status,
		TradeNo:    p.TradeNo,
		CreateTime: p.CreateTime,
	}
}

// callbackPayload 回调报文
type callbackPayload struct {
	PaymentNo string `json:"paymentNo"`
	TradeNo   string `json:"tradeNo"`
	Result    string `json:"result"`
}

// publishPaymentNotify 发送支付结果 MQ 通知（不阻塞主流程）
func publishPaymentNotify(ctx context.Context, svcCtx *svc.ServiceContext, logger logx.Logger, p model.Payment, action string) {
	if svcCtx.MqProducer == nil {
		return
	}
	if err := svcCtx.MqProducer.Publish(ctx, mq.PaymentNotify{
		PaymentNo: p.PaymentNo,
		UserId:    p.UserId,
		OrderType: p.OrderType,
		OrderNo:   p.OrderNo,
		Amount:    p.Amount,
		Action:    action,
	}); err != nil {
		logger.Errorf("发送支付通知失败，不影响主流程: %v", err)
	}
}

// processCallback 处理支付回调通用逻辑
func processCallback(ctx context.Context, svcCtx *svc.ServiceContext, logger logx.Logger, rawBody, expectedChannel string) (*types.CallbackResp, error) {
	var payload callbackPayload
	if err := json.Unmarshal([]byte(rawBody), &payload); err != nil || payload.PaymentNo == "" {
		return nil, common.ErrParam
	}
	if payload.Result == "" {
		payload.Result = "success"
	}

	p, err := svcCtx.PaymentModel.FindByPaymentNo(ctx, payload.PaymentNo)
	if err != nil {
		return nil, common.ErrPaymentNotFound
	}

	// 幂等
	if p.Status == model.PaymentStatusSuccess {
		return &types.CallbackResp{Code: "SUCCESS", Msg: "OK"}, nil
	}

	// Redis 回调去重（防止网关重试）
	if payload.TradeNo != "" {
		dedupKey := "pay:callback:" + expectedChannel + ":" + payload.TradeNo
		ok, _ := svcCtx.Redis.SetnxEx(dedupKey, "1", 604800) // 7天
		if !ok {
			return &types.CallbackResp{Code: "SUCCESS", Msg: "OK"}, nil // 已处理过的回调
		}
	}

	if p.Channel != expectedChannel {
		return nil, common.NewBizError(common.ErrParam.Code, "支付渠道与回调不匹配")
	}

	targetStatus := model.PaymentStatusSuccess
	if payload.Result != "success" {
		targetStatus = model.PaymentStatusFailed
	}
	if !model.CanPaymentTransit(p.Status, targetStatus) {
		return nil, common.ErrStatusInvalid
	}

	updated, err := svcCtx.PaymentModel.UpdateStatus(ctx, p.Id, targetStatus, payload.TradeNo)
	if err != nil {
		logger.Errorf("更新支付状态失败: %v", err)
		return nil, common.ErrSystem
	}

	// 记录回调日志
	if logErr := svcCtx.PaymentLogModel.Insert(ctx, &model.PaymentLog{
		PaymentId: p.Id,
		Action:    "callback",
		Request:   rawBody,
		Response:  targetStatus,
	}); logErr != nil {
		logger.Errorf("记录回调日志失败: %v", logErr)
	}

	action := "success"
	if targetStatus == model.PaymentStatusFailed {
		action = "failed"
	}
	publishPaymentNotify(ctx, svcCtx, logger, *updated, action)

	return &types.CallbackResp{Code: "SUCCESS", Msg: "OK"}, nil
}
