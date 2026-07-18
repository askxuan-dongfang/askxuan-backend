package rpc

import (
	"fmt"

	"github.com/askxuan/common/rpc/shoporder"
	"github.com/askxuan/order-service/internal/config"
	"github.com/askxuan/order-service/internal/svc"
	"github.com/askxuan/order-service/rpc/internal/server"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
)

func MustStartOrderRpcServer(c config.Config, svcCtx *svc.ServiceContext) *zrpc.RpcServer {
	s := zrpc.MustNewServer(c.OrderRpc, func(gs *grpc.Server) {
		shoporder.RegisterShopOrderServiceServer(gs, server.NewShopOrderServer(svcCtx))
	})
	go s.Start()
	fmt.Printf("启动 order-service gRPC，监听 %s\n", c.OrderRpc.ListenOn)
	return s
}
