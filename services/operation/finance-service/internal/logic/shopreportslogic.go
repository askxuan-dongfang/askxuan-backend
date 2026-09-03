package logic

import (
	"context"

	"github.com/askxuan/finance-service/internal/model"
	"github.com/askxuan/finance-service/internal/svc"
	"github.com/askxuan/finance-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// ShopReportsLogic 商城报表逻辑
type ShopReportsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewShopReportsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ShopReportsLogic {
	return &ShopReportsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ShopReports 商城维度财务兼容报表。商品排行由订单域接口提供，此处不伪造商品数据。
func (l *ShopReportsLogic) ShopReports(req *types.ShopReportReq) (*types.ShopReportResp, error) {
	gross, refunds, totalOrders, refundOrders, trendRows, err := model.QueryShopReport(l.ctx, req.StartTime, req.EndTime)
	if err != nil {
		l.Errorf("查询商城财务报表失败: %v", err)
		return nil, err
	}
	totalSales := gross - refunds
	var avgOrderValue float64
	if totalOrders > 0 {
		avgOrderValue = totalSales / float64(totalOrders)
	}
	var refundRate float64
	if totalOrders > 0 {
		refundRate = float64(refundOrders) / float64(totalOrders)
	}
	salesTrend := make([]types.SalesTrendPoint, 0, len(trendRows))
	for _, row := range trendRows {
		salesTrend = append(salesTrend, types.SalesTrendPoint{Date: row.Date, Sales: row.Sales, Orders: row.Orders})
	}

	return &types.ShopReportResp{
		TotalSales:    totalSales,
		TotalOrders:   totalOrders,
		AvgOrderValue: avgOrderValue,
		RefundRate:    refundRate,
		SalesTrend:    salesTrend,
		TopProducts:   []types.TopProduct{},
	}, nil
}
