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

// RefundLogic 发起退款
type RefundLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRefundLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefundLogic {
	return &RefundLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// Refund 发起退款
func (l *RefundLogic) Refund(req *types.RefundReq) (*types.RefundResp, error) {
	if req.PaymentNo == "" || req.Amount <= 0 {
		return nil, common.ErrParam
	}

	p, err := l.svcCtx.PaymentModel.FindByPaymentNo(l.ctx, req.PaymentNo)
	if err != nil {
		return nil, common.ErrPaymentNotFound
	}

	created, err := model.AtomicMockRefund(l.ctx, l.svcCtx.DB, p.Id, req.Amount, req.Reason)
	if err != nil {
		return nil, err
	}
	refunded, err := l.svcCtx.PaymentModel.FindByPaymentNo(l.ctx, p.PaymentNo)
	if err != nil {
		return nil, common.ErrSystem
	}

	// 记录退款日志
	if logErr := l.svcCtx.PaymentLogModel.Insert(l.ctx, &model.PaymentLog{
		PaymentId: p.Id,
		Action:    "refund",
		Request:   fmt.Sprintf("paymentNo=%s,amount=%.2f,reason=%s", req.PaymentNo, req.Amount, req.Reason),
		Response:  model.PaymentStatusRefunded,
	}); logErr != nil {
		l.Errorf("记录退款日志失败: %v", logErr)
	}

	publishPaymentNotify(l.ctx, l.svcCtx, l.Logger, *refunded, "refunded")

	return &types.RefundResp{
		Id:       created.Id,
		RefundNo: created.RefundNo,
		Status:   model.RefundStatusSuccess,
	}, nil
}
