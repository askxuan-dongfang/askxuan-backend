package logic

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/payment-service/internal/model"
	"github.com/askxuan/payment-service/internal/svc"
	"github.com/askxuan/payment-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
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
	if !isValidChannel(req.Channel, l.svcCtx.Config.Provider) || !isValidOrderType(req.OrderType) {
		return nil, common.ErrParam
	}
	userID := strconv.FormatInt(middleware.UserIDFromCtx(l.ctx), 10)
	if userID == "0" {
		return nil, common.ErrUnauthorized
	}
	req.UserId = userID
	if req.OrderType == model.OrderTypeDiyOrder {
		if err := l.svcCtx.DiyOrderModel.ValidatePayment(l.ctx, req.OrderNo, req.UserId, req.Amount); err != nil {
			switch {
			case errors.Is(err, model.ErrDiyOrderOwnerMismatch):
				return nil, common.NewBizError(common.ErrForbidden.Code, "无权支付该DIY订单")
			case errors.Is(err, model.ErrDiyOrderNotPayable):
				return nil, common.NewBizError(common.ErrOrderStatusConflict.Code, "DIY订单当前状态不可支付")
			case errors.Is(err, model.ErrDiyOrderAmountChanged):
				return nil, common.NewBizError(common.ErrOrderStatusConflict.Code, "订单金额已变化，请刷新后重试")
			default:
				l.Errorf("校验DIY订单支付信息失败: %v", err)
				return nil, common.ErrSystem
			}
		}
	}
	if req.OrderType == model.OrderTypeShopOrder {
		if _, err := l.svcCtx.ShopOrderClient.ValidatePayment(l.ctx, req.OrderNo, req.UserId, req.Amount); err != nil {
			l.Errorf("校验商城订单支付信息失败: %v", err)
			return nil, common.NewBizError(common.ErrOrderStatusConflict.Code, "商城订单归属、状态或金额校验失败")
		}
	}

	idempotencyKey := req.OrderType + ":" + req.OrderNo
	if existing, err := l.svcCtx.PaymentModel.FindByIdempotencyKey(l.ctx, idempotencyKey); err == nil {
		return &types.PaymentCreateResp{Id: existing.Id, PaymentNo: existing.PaymentNo,
			PayUrl: fmt.Sprintf("https://mock-pay.example.com/pay/%s?channel=%s", existing.PaymentNo, existing.Channel)}, nil
	} else if !errors.Is(err, sqlx.ErrNotFound) {
		return nil, common.ErrSystem
	}

	created, err := l.svcCtx.PaymentModel.Insert(l.ctx, &model.Payment{
		IdempotencyKey: idempotencyKey,
		UserId:         req.UserId,
		OrderType:      req.OrderType,
		OrderNo:        req.OrderNo,
		Amount:         req.Amount,
		Channel:        req.Channel,
		Status:         model.PaymentStatusPending,
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

	if req.Channel == model.PaymentChannelMock {
		updated, updateErr := l.svcCtx.PaymentModel.UpdateStatus(l.ctx, created.Id, model.PaymentStatusSuccess, "MOCK-"+created.PaymentNo)
		if updateErr != nil {
			return nil, common.ErrSystem
		}
		_ = l.svcCtx.PaymentLogModel.Insert(l.ctx, &model.PaymentLog{PaymentId: created.Id, Action: "mock_success", Request: idempotencyKey, Response: model.PaymentStatusSuccess})
		publishPaymentNotify(l.ctx, l.svcCtx, l.Logger, *updated, "success")
		created = updated
	}

	// 写入幂等标记（2 小时），同一订单重复创建支付单时可据此识别
	_, _ = l.svcCtx.Redis.SetnxEx("pay:idem:"+req.OrderNo, strconv.FormatInt(created.Id, 10), 7200)

	payUrl := fmt.Sprintf("https://mock-pay.example.com/pay/%s?channel=%s", created.PaymentNo, created.Channel)

	return &types.PaymentCreateResp{
		Id:        created.Id,
		PaymentNo: created.PaymentNo,
		PayUrl:    payUrl,
	}, nil
}
