package logic

import (
	"context"
	"time"

	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/booking-service/internal/types"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/logx"
)

type AvailabilityLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAvailabilityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AvailabilityLogic {
	return &AvailabilityLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}
func (l *AvailabilityLogic) Availability(req *types.AvailabilityReq) (*types.AvailabilityResp, error) {
	date, err := time.ParseInLocation("2006-01-02", req.BookingDate, time.Local)
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	if req.TempleId == "" || req.ServiceId == "" || err != nil || date.Before(today) {
		return nil, common.ErrParamInvalid
	}
	service, err := l.svcCtx.TempleClient.GetBookingService(l.ctx, req.TempleId, req.ServiceId)
	if err != nil {
		return nil, mapTempleRpcError(err)
	}
	usage, err := l.svcCtx.BookingModel.FindSlotUsage(l.ctx, req.TempleId, req.ServiceId, req.BookingDate)
	if err != nil {
		return nil, common.ErrSystem
	}
	slots := make([]types.AvailableSlot, 0, len(service.Slots))
	for _, slot := range service.Slots {
		remaining := int(slot.Capacity) - usage[slot.Code]
		if remaining < 0 {
			remaining = 0
		}
		enabled := slot.Status == "enabled" && remaining > 0
		slots = append(slots, types.AvailableSlot{SlotCode: slot.Code, Label: slot.Label, TimeRange: slot.StartTime + "-" + slot.EndTime, Capacity: int(slot.Capacity), Remaining: remaining, Available: enabled})
	}
	return &types.AvailabilityResp{TempleId: service.TempleCode, ServiceId: service.ServiceCode, ServiceName: service.ServiceName, BookingDate: req.BookingDate, ServiceFee: service.Price, Slots: slots}, nil
}
