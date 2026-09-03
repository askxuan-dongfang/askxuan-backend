package logic

import (
	"context"

	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/booking-service/internal/types"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/zeromicro/go-zero/core/logx"
)

type AdminBookingReportLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminBookingReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminBookingReportLogic {
	return &AdminBookingReportLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminBookingReportLogic) Report(req *types.AdminBookingReportReq) (*types.AdminBookingReportResp, error) {
	if req.TempleId == "" {
		return nil, common.ErrParam
	}
	for _, role := range middleware.RolesFromCtx(l.ctx) {
		if role == "temple_admin" {
			if code := middleware.TempleCodeFromCtx(l.ctx); code == "" || code != req.TempleId {
				return nil, common.ErrTempleIsolation
			}
			break
		}
	}
	stats, trend, services, masters, err := l.svcCtx.BookingModel.Report(l.ctx, req.TempleId, req.StartTime, req.EndTime)
	if err != nil {
		l.Errorf("查询寺院预约报表失败: %v", err)
		return nil, common.ErrSystem
	}
	resp := &types.AdminBookingReportResp{
		BookingTrend:        make([]types.BookingTrendPoint, 0, len(trend)),
		RevenueStats:        types.BookingRevenueStats{TotalRevenue: stats.MeritMoney, BookingCount: stats.BookingCount, CompletedCount: stats.CompletedCount},
		ServiceDistribution: make([]types.BookingServiceDist, 0, len(services)), MasterRanking: make([]types.BookingMasterRank, 0, len(masters)),
	}
	if stats.BookingCount > 0 {
		resp.RevenueStats.AvgBookingValue = stats.MeritMoney / float64(stats.BookingCount)
	}
	for _, row := range trend {
		resp.BookingTrend = append(resp.BookingTrend, types.BookingTrendPoint{Date: row.Date, Bookings: row.Bookings, Revenue: row.MeritMoney})
	}
	for _, row := range services {
		percentage := 0.0
		if stats.BookingCount > 0 {
			percentage = float64(row.Count) / float64(stats.BookingCount)
		}
		resp.ServiceDistribution = append(resp.ServiceDistribution, types.BookingServiceDist{ServiceName: row.ServiceName, Count: row.Count, Percentage: percentage})
	}
	for _, row := range masters {
		resp.MasterRanking = append(resp.MasterRanking, types.BookingMasterRank{MasterCode: row.MasterCode, MasterName: row.MasterName, BookingCount: row.Count, Revenue: row.MeritMoney})
	}
	return resp, nil
}
