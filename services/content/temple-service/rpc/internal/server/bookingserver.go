package server

import (
	"context"

	"github.com/askxuan/temple-service/internal/svc"
	"github.com/askxuan/temple-service/rpc/temple"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TempleBookingServer struct {
	temple.UnimplementedTempleBookingServiceServer
	svcCtx *svc.ServiceContext
}

func NewTempleBookingServer(svcCtx *svc.ServiceContext) *TempleBookingServer {
	return &TempleBookingServer{svcCtx: svcCtx}
}

func (s *TempleBookingServer) GetBookingService(ctx context.Context, req *temple.GetBookingServiceReq) (*temple.BookingService, error) {
	t, err := s.svcCtx.TempleModel.FindOne(ctx, req.TempleCode)
	if err != nil {
		if err == sqlx.ErrNotFound {
			return nil, status.Error(codes.NotFound, "temple not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	service, err := s.svcCtx.TempleServiceModel.FindByCodes(ctx, req.TempleCode, req.ServiceCode)
	if err != nil {
		if err == sqlx.ErrNotFound {
			return nil, status.Error(codes.NotFound, "service not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	slots := make([]*temple.BookingSlot, 0, len(service.Slots))
	for _, slot := range service.Slots {
		slots = append(slots, &temple.BookingSlot{Code: slot.Code, Label: slot.Label, StartTime: slot.StartTime, EndTime: slot.EndTime, Capacity: int64(slot.Capacity), Status: slot.Status})
	}
	return &temple.BookingService{TempleCode: t.Code, TempleName: t.Name, TempleStatus: t.Status, ServiceCode: service.ServiceCode, ServiceName: service.ServiceName, Price: service.Price, ServiceStatus: service.Status, Slots: slots}, nil
}
