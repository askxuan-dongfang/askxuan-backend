package logic

import (
	"context"
	"fmt"
	"strconv"

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

	if !model.CanPaymentTransit(p.Status, model.PaymentStatusRefunding) {
		return nil, common.ErrStatusInvalid
	}
	if req.Amount > p.Amount {
		return nil, common.NewBizError(common.ErrParam.Code, "退款金额超过支付金额")
	}

	// 创建退款单
	created, err := l.svcCtx.RefundModel.Insert(l.ctx, &model.Refund{
		PaymentId: p.Id,
		Amount:    req.Amount,
		Reason:    req.Reason,
		Status:    model.RefundStatusProcessing,
	})
	if err != nil {
		l.Errorf("创建退款单失败: %v", err)
		return nil, common.ErrSystem
	}
	// 写入退款幂等标记（24 小时），同一退款单重复请求时可据此识别
	_, _ = l.svcCtx.Redis.SetnxEx("pay:refund:idem:"+created.RefundNo, strconv.FormatInt(created.Id, 10), 86400)

	// 支付单状态流转 success → refunding
	if _, err := l.svcCtx.PaymentModel.UpdateStatus(l.ctx, p.Id, model.PaymentStatusRefunding, ""); err != nil {
		l.Errorf("更新支付状态失败: %v", err)
		return nil, common.ErrSystem
	}

	// MVP mock 第三方退款立即成功
	if _, err := l.svcCtx.RefundModel.UpdateStatus(l.ctx, created.Id, model.RefundStatusSuccess); err != nil {
		l.Errorf("更新退款状态失败: %v", err)
		return nil, common.ErrSystem
	}
	refunded, err := l.svcCtx.PaymentModel.UpdateStatus(l.ctx, p.Id, model.PaymentStatusRefunded, "")
	if err != nil {
		l.Errorf("更新支付状态失败: %v", err)
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
