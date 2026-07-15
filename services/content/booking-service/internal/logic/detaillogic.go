package logic

import (
	"context"
	"errors"

	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/booking-service/internal/types"
	"github.com/askxuan/common"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// DetailLogic 预约详情查询逻辑
type DetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DetailLogic {
	return &DetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Detail 按 ID 查询预约详情
func (l *DetailLogic) Detail(req *types.DetailReq) (*types.Booking, error) {
	userID, authErr := authenticatedUserID(l.ctx)
	if authErr != nil {
		return nil, authErr
	}
	b, err := l.svcCtx.BookingModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrBookingNotFound
		}
		l.Errorf("查询预约详情失败: %v", err)
		return nil, common.ErrSystem
	}
	if b.UserId != userID {
		return nil, common.ErrForbidden
	}
	resp := types.Booking(*b)
	return &resp, nil
}
