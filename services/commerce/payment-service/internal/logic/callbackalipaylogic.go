package logic

import (
	"context"

	"github.com/askxuan/payment-service/internal/model"
	"github.com/askxuan/payment-service/internal/svc"
	"github.com/askxuan/payment-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// CallbackAlipayLogic 支付宝回调
type CallbackAlipayLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCallbackAlipayLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CallbackAlipayLogic {
	return &CallbackAlipayLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// Callback 支付宝支付回调
// 验证报文 → 幂等校验 → 状态流转 pending→success/failed → 记录日志 → 发 MQ 通知
func (l *CallbackAlipayLogic) Callback(req *types.CallbackAlipayReq) (*types.CallbackResp, error) {
	return processCallback(l.ctx, l.svcCtx, l.Logger, req.RawBody, model.PaymentChannelAlipay)
}
