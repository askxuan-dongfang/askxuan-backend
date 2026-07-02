package logic

import (
	"context"
	"fmt"

	"github.com/askxuan/common"
	"github.com/askxuan/payment-service/internal/model"
	"github.com/askxuan/payment-service/internal/svc"
	"github.com/askxuan/payment-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// PaymentCreateLogic 创建支付单
type PaymentCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPaymentCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PaymentCreateLogic {
	return &PaymentCreateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// Create 创建支付单
func (l *PaymentCreateLogic) Create(req *types.PaymentCreateReq) (*types.PaymentCreateResp, error) {
	if req.OrderNo == "" || req.Amount <= 0 {
		return nil, common.ErrParam
	}
	if !isValidChannel(req.Channel) || !isValidOrderType(req.OrderType) {
		return nil, common.ErrParam
	}

	created, err := l.svcCtx.PaymentModel.Insert(l.ctx, &model.Payment{
		UserId:    req.UserId,
		OrderType: req.OrderType,
		OrderNo:   req.OrderNo,
		Amount:    req.Amount,
		Channel:   req.Channel,
		Status:    model.PaymentStatusPending,
	})
	if err != nil {
		l.Errorf("创建支付单失败: %v", err)
		return nil, common.ErrSystem
	}

	// 记录创建日志
	if logErr := l.svcCtx.PaymentLogModel.Insert(l.ctx, &model.PaymentLog{
		PaymentId: created.Id,
		Action:    "create",
		Request:   fmt.Sprintf("orderType=%s,orderNo=%s,amount=%.2f,channel=%s", req.OrderType, req.OrderNo, req.Amount, req.Channel),
		Response:  model.PaymentStatusPending,
	}); logErr != nil {
		l.Errorf("记录支付日志失败: %v", logErr)
	}

	payUrl := fmt.Sprintf("https://mock-pay.example.com/pay/%s?channel=%s", created.PaymentNo, created.Channel)

	return &types.PaymentCreateResp{
		Id:        created.Id,
		PaymentNo: created.PaymentNo,
		PayUrl:    payUrl,
	}, nil
}
