package logic

import (
	"context"

	"github.com/askxuan/booking-service/internal/model"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/booking-service/internal/types"
	"github.com/askxuan/common"

	"github.com/zeromicro/go-zero/core/logx"
)

// UpdateStatusLogic 预约状态流转逻辑
type UpdateStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateStatusLogic {
	return &UpdateStatusLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// UpdateStatus 预约状态流转（C 端）
// 合法流转：pending → confirmed/cancelled，confirmed → in_progress/cancelled，
//
//	in_progress → completed/cancelled，completed → reviewed
func (l *UpdateStatusLogic) UpdateStatus(req *types.StatusReq) (*types.StatusResp, error) {
	if req.Status != model.StatusCancelled {
		return nil, common.ErrParam
	}
	userID, err := authenticatedUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	booking, err := l.svcCtx.BookingModel.FindOne(l.ctx, req.Id)
	if err != nil {
		return nil, common.ErrBookingNotFound
	}
	if booking.UserId != userID {
		return nil, common.ErrForbidden
	}
	return transitBookingStatus(l.Logger, l.ctx, l.svcCtx, req.Id, req.Status, "", userID, model.OperatorTypeUser, req.Status)
}
