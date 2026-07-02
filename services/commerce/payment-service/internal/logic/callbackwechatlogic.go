package logic

import (
	"context"

	"github.com/askxuan/payment-service/internal/model"
	"github.com/askxuan/payment-service/internal/svc"
	"github.com/askxuan/payment-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// CallbackWechatLogic 微信回调
type CallbackWechatLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCallbackWechatLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CallbackWechatLogic {
	return &CallbackWechatLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// Callback 微信支付回调
// 验证报文 → 幂等校验 → 状态流转 pending→success/failed → 记录日志 → 发 MQ 通知
func (l *CallbackWechatLogic) Callback(req *types.CallbackWechatReq) (*types.CallbackResp, error) {
	return processCallback(l.ctx, l.svcCtx, l.Logger, req.RawBody, model.PaymentChannelWechat)
}
