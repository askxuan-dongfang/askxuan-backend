package server

import (
	"context"
	"math"

	"github.com/askxuan/common/rpc/shoporder"
	"github.com/askxuan/order-service/internal/model"
	"github.com/askxuan/order-service/internal/svc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ShopOrderServer struct {
	shoporder.UnimplementedShopOrderServiceServer
	svcCtx *svc.ServiceContext
}

func NewShopOrderServer(svcCtx *svc.ServiceContext) *ShopOrderServer {
	return &ShopOrderServer{svcCtx: svcCtx}
}

func (s *ShopOrderServer) ValidatePayment(ctx context.Context, req *shoporder.ValidatePaymentReq) (*shoporder.ValidatedShopOrder, error) {
	o, err := s.svcCtx.ShopOrderModel.FindByOrderNo(ctx, req.GetOrderNo())
	if err != nil {
		if err == sqlx.ErrNotFound {
			return nil, status.Error(codes.NotFound, "商城订单不存在")
		}
		return nil, status.Error(codes.Internal, "商城订单查询失败")
	}
	if o.UserId != req.GetUserId() {
		return nil, status.Error(codes.PermissionDenied, "无权支付该订单")
	}
	if o.Status != model.OrderStatusPendingPayment {
		return nil, status.Error(codes.FailedPrecondition, "商城订单当前状态不可支付")
	}
	if math.Abs(o.PayAmount-req.GetAmount()) > 0.001 {
		return nil, status.Error(codes.FailedPrecondition, "订单金额已变化")
	}
	return &shoporder.ValidatedShopOrder{Id: o.Id, OrderNo: o.OrderNo, UserId: o.UserId, PayAmount: o.PayAmount, Status: o.Status}, nil
}
