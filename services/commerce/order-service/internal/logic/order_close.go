package logic

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/order-service/internal/svc"
	"github.com/askxuan/order-service/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
)

func CancelPendingOrder(ctx context.Context, service *svc.ServiceContext, id int64) (*types.ShopOrder, error) {
	userID := middleware.UserIDFromCtx(ctx)
	if userID <= 0 {
		return nil, common.ErrUnauthorized
	}
	if err := service.ShopOrderModel.ClosePending(ctx, id, strconv.FormatInt(userID, 10)); err != nil {
		var business *common.BizError
		if errors.As(err, &business) {
			return nil, business
		}
		logx.WithContext(ctx).Errorf("取消商城订单失败 id=%d: %v", id, err)
		return nil, common.ErrSystem
	}
	return NewOrderDetailLogic(ctx, service).Detail(&types.OrderDetailReq{Id: id})
}

func RunPendingOrderMaintenance(ctx context.Context, service *svc.ServiceContext) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		maintainPendingOrders(ctx, service)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func maintainPendingOrders(ctx context.Context, service *svc.ServiceContext) {
	const batchSize = 100
	orders := service.ShopOrderModel
	ids, err := orders.FindExpired(ctx, batchSize)
	if err != nil {
		logx.Errorf("查询超时商城订单失败: %v", err)
	}
	for _, id := range ids {
		err := orders.ClosePending(ctx, id, "")
		var business *common.BizError
		if err != nil && !errors.As(err, &business) {
			logx.Errorf("关闭超时商城订单失败 id=%d: %v", id, err)
		}
	}
	releases, err := orders.FindStockReleases(ctx, batchSize)
	if err != nil {
		logx.Errorf("查询待回补库存的商城订单失败: %v", err)
		return
	}
	for _, release := range releases {
		callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := service.CatalogClient.ReleaseCart(callCtx, release.RequestID)
		cancel()
		if err != nil {
			logx.Errorf("回补商城订单库存失败 id=%d: %v", release.ID, err)
			continue
		}
		if err := orders.CompleteStockRelease(ctx, release.ID); err != nil {
			logx.Errorf("记录商城库存回补失败 id=%d: %v", release.ID, err)
		}
	}
}
