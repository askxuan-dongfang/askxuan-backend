package logic

import (
	"context"
	"errors"
	"time"

	"github.com/askxuan/booking-service/internal/model"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/booking-service/internal/types"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type PayLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewPayLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PayLogic {
	return &PayLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}
func (l *PayLogic) Pay(req *types.PayReq) (*types.CreateResp, error) {
	b, err := l.svcCtx.BookingModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrBookingNotFound
		}
		return nil, common.ErrSystem
	}
	userID, authErr := authenticatedUserID(l.ctx)
	if authErr != nil {
		return nil, authErr
	}
	if b.UserId != userID {
		return nil, common.ErrForbidden
	}
	if b.PaymentStatus == model.PaymentStatusSuccess {
		return responseFromBooking(b), nil
	}
	expires, _ := time.ParseInLocation("2006-01-02 15:04:05", b.PaymentExpireTime, time.Local)
	if b.Status != model.StatusPendingPayment || expires.Before(time.Now()) {
		return nil, common.ErrBookingPaymentExpired
	}
	return responseFromBooking(NewCreateLogic(l.ctx, l.svcCtx).autoPay(b)), nil
}
