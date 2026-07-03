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

// ShopReports 商城维度财务报表
// 聚合 settlement（SettleType=shop）与 finance_log 数据，趋势与热销商品使用内存 mock
func (l *ShopReportsLogic) ShopReports(req *types.ShopReportReq) (*types.ShopReportResp, error) {
	// 从结算单聚合商城销售额与订单数
	settlements, _ := model.ListSettlements(model.SettleTypeShop, "", 1, 1000)
	var totalSales float64
	var totalOrders int
	for _, s := range settlements {
		totalSales += s.TotalAmount
		totalOrders += s.OrderCount
	}

	// 平均客单价
	var avgOrderValue float64
	if totalOrders > 0 {
		avgOrderValue = totalSales / float64(totalOrders)
	}

	// 退款率：finance_log 中 type=refund 占 income 的比例（无数据时为 0）
	financeSums := model.SumByType(req.StartTime, req.EndTime)
	income := financeSums["income"]
	refund := financeSums["refund"]
	var refundRate float64
	if income > 0 {
		refundRate = refund / income
	}

	// 销售趋势（近 7 天 mock，基于 totalSales 等比缩放）
	salesTrend := buildShopSalesTrend(totalSales)

	// 热销商品 Top5（mock）
	topProducts := buildTopProducts()

	return &types.ShopReportResp{
		TotalSales:    totalSales,
		TotalOrders:   totalOrders,
		AvgOrderValue: avgOrderValue,
		RefundRate:    refundRate,
		SalesTrend:    salesTrend,
		TopProducts:   topProducts,
	}, nil
}

// buildShopSalesTrend 构造近 7 天销售趋势（mock）
func buildShopSalesTrend(totalSales float64) []types.SalesTrendPoint {
	days := []string{"2026-06-26", "2026-06-27", "2026-06-28", "2026-06-29", "2026-06-30", "2026-07-01", "2026-07-02"}
	weights := []float64{0.10, 0.12, 0.15, 0.13, 0.18, 0.20, 0.12}
	points := make([]types.SalesTrendPoint, 0, len(days))
	for i, d := range days {
		sales := totalSales * weights[i] // weights 已归一化，总和≈1.0
		orders := int(sales/300) + 1
		points = append(points, types.SalesTrendPoint{
			Date:   d,
			Sales:  sales,
			Orders: orders,
		})
	}
	return points
}

// buildTopProducts 构造热销商品（mock）
func buildTopProducts() []types.TopProduct {
	return []types.TopProduct{
		{ProductId: 1001, ProductName: "檀木手串-基础款", Sales: 8600.00, OrderCount: 43},
		{ProductId: 1002, ProductName: "翡翠吊坠-平安扣", Sales: 7200.00, OrderCount: 24},
		{ProductId: 1003, ProductName: "沉香线香礼盒", Sales: 5400.00, OrderCount: 36},
		{ProductId: 1004, ProductName: "黄铜莲花香插", Sales: 3200.00, OrderCount: 64},
		{ProductId: 1005, ProductName: "紫檀佛珠-108颗", Sales: 2800.00, OrderCount: 14},
	}
}
