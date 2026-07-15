package rpc

import (
	"fmt"

	"github.com/askxuan/temple-service/internal/config"
	"github.com/askxuan/temple-service/internal/svc"
	"github.com/askxuan/temple-service/rpc/internal/server"
	"github.com/askxuan/temple-service/rpc/temple"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
)

func MustStartTempleRpcServer(c config.Config, svcCtx *svc.ServiceContext) *zrpc.RpcServer {
	s := zrpc.MustNewServer(c.TempleRpc, func(gs *grpc.Server) {
		temple.RegisterTempleBookingServiceServer(gs, server.NewTempleBookingServer(svcCtx))
	})
	go s.Start()
	fmt.Printf("启动 temple-service gRPC，监听 %s\n", c.TempleRpc.ListenOn)
	return s
}
