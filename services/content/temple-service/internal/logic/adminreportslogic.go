package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/temple-service/internal/model"
	"github.com/askxuan/temple-service/internal/svc"
	"github.com/askxuan/temple-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// AdminTempleReportsLogic 寺院报表逻辑（寺院维度，从 JWT TempleID 取当前寺院）
type AdminTempleReportsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminTempleReportsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminTempleReportsLogic {
	return &AdminTempleReportsLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// AdminTempleReports 寺院维度报表
// 聚合本寺院的加持任务（blessing_task）数据作为预约/收入来源，趋势与分布图表使用内存 mock
func (l *AdminTempleReportsLogic) AdminTempleReports(req *types.TempleReportReq) (*types.TempleReportResp, error) {
	t, err := getCurrentTemple(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}

	// 聚合本寺院的加持任务（按 status 拉取全量，page=1 size=1000）
	tasks, total, err := l.svcCtx.BlessingTaskModel.FindByTempleId(l.ctx, t.Code, "", 1, 1000)
	if err != nil {
		l.Errorf("查询寺院加持任务失败: %v", err)
		return nil, common.ErrSystem
	}

	// 统计：总预约数、已完成数、收入（按已完成任务估算，单价 300）
	bookingCount := int(total)
	completedCount := 0
	masterStats := map[string]*masterStat{}
	for _, task := range tasks {
		if task.Status == model.BlessingTaskStatusCompleted {
			completedCount++
		}
		if _, ok := masterStats[task.MasterCode]; !ok {
			masterStats[task.MasterCode] = &masterStat{masterCode: task.MasterCode}
		}
		masterStats[task.MasterCode].bookingCount++
		if task.Status == model.BlessingTaskStatusCompleted {
			masterStats[task.MasterCode].revenue += 300.0
		}
	}
	totalRevenue := float64(completedCount) * 300.0
	var avgBookingValue float64
	if bookingCount > 0 {
		avgBookingValue = totalRevenue / float64(bookingCount)
	}

	// 预约趋势（近 7 天 mock）
	bookingTrend := buildTempleBookingTrend(bookingCount)

	// 服务分布（mock，基于本寺院服务列表）
	services, _ := l.svcCtx.TempleServiceModel.FindByTempleId(l.ctx, t.Code)
	serviceDistribution := buildServiceDistribution(services, bookingCount)

	// 法师排名（从 masterStats 聚合）
	masterRanking := buildMasterRanking(masterStats)

	return &types.TempleReportResp{
		BookingTrend: bookingTrend,
		RevenueStats: types.TempleRevenueStats{
			TotalRevenue:    totalRevenue,
			BookingCount:    bookingCount,
			AvgBookingValue: avgBookingValue,
			CompletedCount:  completedCount,
		},
		ServiceDistribution: serviceDistribution,
		MasterRanking:       masterRanking,
	}, nil
}

type masterStat struct {
	masterCode   string
	bookingCount int
	revenue      float64
}

// buildTempleBookingTrend 构造近 7 天预约趋势（mock）
func buildTempleBookingTrend(totalBookings int) []types.BookingTrendPoint {
	days := []string{"2026-06-26", "2026-06-27", "2026-06-28", "2026-06-29", "2026-06-30", "2026-07-01", "2026-07-02"}
	weights := []float64{0.10, 0.12, 0.15, 0.13, 0.18, 0.20, 0.12}
	points := make([]types.BookingTrendPoint, 0, len(days))
	weightSum := 0.0
	for _, w := range weights {
		weightSum += w
	}
	for i, d := range days {
		bookings := int(float64(totalBookings) * weights[i] / weightSum)
		revenue := float64(bookings) * 300.0
		points = append(points, types.BookingTrendPoint{
			Date:     d,
			Bookings: bookings,
			Revenue:  revenue,
		})
	}
	return points
}

// buildServiceDistribution 构造服务分布（基于寺院服务列表 mock 计数）
func buildServiceDistribution(services []*model.TempleServiceRecord, totalBookings int) []types.ServiceDistItem {
	if len(services) == 0 {
		return []types.ServiceDistItem{}
	}
	items := make([]types.ServiceDistItem, 0, len(services))
	// 均分预约数作为 mock
	perService := totalBookings / len(services)
	if perService < 1 {
		perService = 1
	}
	remaining := totalBookings - perService*len(services)
	for i, s := range services {
		count := perService
		if i < remaining {
			count++
		}
		var pct float64
		if totalBookings > 0 {
			pct = float64(count) / float64(totalBookings)
		}
		items = append(items, types.ServiceDistItem{
			ServiceName: s.ServiceName,
			Count:       count,
			Percentage:  pct,
		})
	}
	return items
}

// buildMasterRanking 构造法师排名
func buildMasterRanking(stats map[string]*masterStat) []types.MasterRankItem {
	items := make([]types.MasterRankItem, 0, len(stats))
	for _, s := range stats {
		items = append(items, types.MasterRankItem{
			MasterCode:   s.masterCode,
			MasterName:   masterNameByCode(s.masterCode),
			BookingCount: s.bookingCount,
			Revenue:      s.revenue,
		})
	}
	// 按 bookingCount 降序（简单冒泡，数据量小）
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].BookingCount > items[i].BookingCount {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
	return items
}

// masterNameByCode 法师编码 → 法师名（mock）
func masterNameByCode(code string) string {
	names := map[string]string{
		"M001": "智海法师",
		"M002": "清风道长",
		"M003": "释延心法师",
		"M004": "扎西多吉活佛",
		"M005": "慧明法师",
		"M006": "真武道长",
	}
	if name, ok := names[code]; ok {
		return name
	}
	return code
}
