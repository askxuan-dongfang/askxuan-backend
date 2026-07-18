package logic

import (
	"context"
	"errors"
	"strconv"

	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/payment-service/internal/svc"
	"github.com/askxuan/payment-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// PaymentQueryLogic 查询支付状态
type PaymentQueryLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPaymentQueryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PaymentQueryLogic {
	return &PaymentQueryLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// Query 查询支付单详情
func (l *PaymentQueryLogic) Query(req *types.PaymentQueryReq) (*types.Payment, error) {
	p, err := l.svcCtx.PaymentModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrPaymentNotFound
		}
		l.Errorf("查询支付单失败: %v", err)
		return nil, common.ErrSystem
	}
	if p.UserId != strconv.FormatInt(middleware.UserIDFromCtx(l.ctx), 10) {
		return nil, common.ErrForbidden
	}
	resp := toTypesPayment(*p)
	return &resp, nil
}
