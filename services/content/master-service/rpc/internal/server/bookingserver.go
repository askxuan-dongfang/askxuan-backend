package server

import (
	"context"

	"github.com/askxuan/master-service/internal/model"
	"github.com/askxuan/master-service/internal/svc"
	"github.com/askxuan/master-service/rpc/master"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type MasterBookingServer struct {
	master.UnimplementedMasterBookingServiceServer
	svcCtx *svc.ServiceContext
}

func NewMasterBookingServer(svcCtx *svc.ServiceContext) *MasterBookingServer {
	return &MasterBookingServer{svcCtx: svcCtx}
}
func (s *MasterBookingServer) GetBookingMaster(ctx context.Context, req *master.GetBookingMasterReq) (*master.BookingMaster, error) {
	var m *model.Master
	var err error
	if req.MasterCode != "" {
		m, err = s.svcCtx.MasterModel.FindByCode(ctx, req.MasterCode)
	} else {
		m, err = s.svcCtx.MasterModel.FindOne(ctx, req.MasterId)
	}
	if err != nil {
		if err == sqlx.ErrNotFound {
			return nil, status.Error(codes.NotFound, "master not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	templeName, _ := s.svcCtx.MasterModel.FindTempleNameByCode(ctx, m.TempleCode)
	return &master.BookingMaster{Id: m.Id, Code: m.Code, DharmaName: m.DharmaName,
		TempleCode: m.TempleCode, TempleName: templeName, ShelfStatus: m.ShelfStatus,
		PlatformStatus: m.PlatformStatus, ConsultEnabled: m.ConsultEnabled,
		ConsultFee: m.ConsultFee, ConsultValidHours: int32(m.ConsultValidHours),
		ConsultResponseMinutes: int32(m.ConsultResponseMinutes)}, nil
}
