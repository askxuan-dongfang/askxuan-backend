package rpc

import (
	"fmt"

	"github.com/askxuan/payment-service/internal/config"
	"github.com/askxuan/payment-service/internal/svc"
	"github.com/askxuan/payment-service/rpc/internal/server"
	"github.com/askxuan/payment-service/rpc/payment"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
)

func MustStartPaymentRpcServer(c config.Config, svcCtx *svc.ServiceContext) *zrpc.RpcServer {
	s := zrpc.MustNewServer(c.PaymentRpc, func(gs *grpc.Server) {
		payment.RegisterPaymentBookingServiceServer(gs, server.NewPaymentBookingServer(svcCtx))
	})
	go s.Start()
	fmt.Printf("启动 payment-service gRPC，监听 %s\n", c.PaymentRpc.ListenOn)
	return s
}
