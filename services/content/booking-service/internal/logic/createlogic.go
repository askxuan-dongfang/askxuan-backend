package logic

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/askxuan/booking-service/internal/model"
	"github.com/askxuan/booking-service/internal/mq"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/booking-service/internal/types"
	"github.com/askxuan/booking-service/rpc/temple"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateLogic {
	return &CreateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *CreateLogic) Create(req *types.CreateReq) (*types.CreateResp, error) {
	authUser, authErr := authenticatedUserID(l.ctx)
	if authErr != nil {
		return nil, authErr
	}
	if req.UserId != "" && req.UserId != authUser {
		return nil, common.ErrForbidden
	}
	req.UserId = authUser
	if req.UserId == "" || req.TempleId == "" || req.MasterId == "" || req.ServiceId == "" || req.BookingDate == "" || (req.SlotCode == "" && req.TimeSlot == "") || req.MeritMoney < 0 {
		return nil, common.ErrParam
	}
	date, err := time.ParseInLocation("2006-01-02", req.BookingDate, time.Local)
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	if err != nil || date.Before(today) {
		return nil, common.ErrParamInvalid
	}
	if req.RequestId != "" {
		if existing, findErr := l.svcCtx.BookingModel.FindByRequest(l.ctx, req.UserId, req.RequestId); findErr == nil {
			return responseFromBooking(existing), nil
		}
	}

	service, err := l.svcCtx.TempleClient.GetBookingService(l.ctx, req.TempleId, req.ServiceId)
	if err != nil {
		return nil, mapTempleRpcError(err)
	}
	if service.TempleStatus != "正常" || service.ServiceStatus != "on_shelf" {
		return nil, common.NewBizError(40310, "寺院服务当前不可预约")
	}
	master, err := l.svcCtx.MasterClient.GetByCode(l.ctx, req.MasterId)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, common.ErrMasterNotFound
		}
		return nil, common.ErrDependencyUnavailable
	}
	if master.ShelfStatus != "on_shelf" || (master.PlatformStatus != "" && master.PlatformStatus != "normal") {
		return nil, common.NewBizError(40308, "法师当前不可预约")
	}
	if master.TempleCode != req.TempleId {
		return nil, common.NewBizError(40309, "法师不属于所选寺院")
	}

	slot := selectSlot(service.Slots, req.SlotCode, req.TimeSlot)
	if slot == nil || slot.Status != "enabled" {
		return nil, common.ErrBookingSlotNotFound
	}
	timeRange := slot.StartTime + "-" + slot.EndTime
	total := service.Price + req.MeritMoney
	snapshot, _ := json.Marshal(map[string]interface{}{"serviceCode": service.ServiceCode, "serviceName": service.ServiceName, "serviceFee": service.Price, "meritMoney": req.MeritMoney, "totalFee": total, "slotCode": slot.Code, "timeSlot": timeRange})
	created, err := l.svcCtx.BookingModel.InsertWithReservation(l.ctx, &model.Booking{RequestId: req.RequestId, UserId: req.UserId, TempleId: service.TempleCode, TempleName: service.TempleName, MasterId: master.Code, MasterName: master.DharmaName, ServiceId: service.ServiceCode, ServiceName: service.ServiceName, BookingDate: req.BookingDate, SlotCode: slot.Code, TimeSlot: timeRange, ServiceFee: service.Price, MeritMoney: req.MeritMoney, MeritMoneyTier: req.MeritMoneyTier, TotalFee: total, PriceSnapshot: string(snapshot), Note: req.Note}, int(slot.Capacity))
	if err != nil {
		if errors.Is(err, model.ErrSlotFull) {
			return nil, common.ErrBookingSlotFull
		}
		if req.RequestId != "" {
			if existing, findErr := l.svcCtx.BookingModel.FindByRequest(l.ctx, req.UserId, req.RequestId); findErr == nil {
				return responseFromBooking(existing), nil
			}
		}
		l.Errorf("创建预约与占位失败: %v", err)
		return nil, common.ErrSystem
	}
	_ = l.svcCtx.StatusLogModel.Insert(l.ctx, &model.BookingStatusLog{BookingId: created.Id, FromStatus: "", ToStatus: model.StatusPendingPayment, OperatorId: req.UserId, OperatorType: model.OperatorTypeUser, Remark: "创建待支付预约"})
	paid := l.autoPay(created)
	return responseFromBooking(paid), nil
}

func (l *CreateLogic) autoPay(booking *model.Booking) *model.Booking {
	payment, err := l.svcCtx.PaymentClient.AutoPayBooking(l.ctx, booking.Id, booking.UserId, booking.TotalFee)
	if err != nil {
		l.Errorf("模拟支付暂不可用 booking=%s: %v", booking.Id, err)
		return booking
	}
	updated, changed, err := l.svcCtx.BookingModel.UpdatePayment(l.ctx, booking.Id, payment.PaymentNo, payment.Channel, payment.Status, model.StatusPending)
	if err != nil {
		l.Errorf("支付成功后更新预约失败 booking=%s: %v", booking.Id, err)
		return booking
	}
	if changed {
		_ = l.svcCtx.StatusLogModel.Insert(l.ctx, &model.BookingStatusLog{BookingId: booking.Id, FromStatus: model.StatusPendingPayment, ToStatus: model.StatusPending, OperatorId: "payment-service", OperatorType: model.OperatorTypeSystem, Remark: "模拟支付成功"})
		if l.svcCtx.MqProducer != nil {
			_ = l.svcCtx.MqProducer.Publish(l.ctx, mq.BookingNotify{
				BookingId: updated.Id, UserId: updated.UserId,
				TempleId: updated.TempleId, TempleName: updated.TempleName,
				MasterId: updated.MasterId, MasterName: updated.MasterName,
				ServiceName: updated.ServiceName, BookingDate: updated.BookingDate,
				ServiceFee: updated.ServiceFee, MeritMoney: updated.MeritMoney,
				TotalFee: updated.TotalFee, Action: "created",
			})
		}
	}
	return updated
}

func selectSlot(slots []*temple.BookingSlot, code, legacyRange string) *temple.BookingSlot {
	for _, slot := range slots {
		if (code != "" && slot.Code == code) || (code == "" && slot.StartTime+"-"+slot.EndTime == legacyRange) {
			return slot
		}
	}
	return nil
}
func mapTempleRpcError(err error) error {
	if status.Code(err) == codes.NotFound {
		return common.ErrTempleServiceNotFound
	}
	return common.ErrDependencyUnavailable
}
func responseFromBooking(b *model.Booking) *types.CreateResp {
	return &types.CreateResp{Id: b.Id, Status: b.Status, PaymentStatus: b.PaymentStatus, PaymentNo: b.PaymentNo, ServiceFee: b.ServiceFee, MeritMoney: b.MeritMoney, TotalFee: b.TotalFee, Simulated: b.PaymentChannel == "mock"}
}
