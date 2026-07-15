package rpc

import (
	"fmt"

	"github.com/askxuan/master-service/internal/config"
	"github.com/askxuan/master-service/internal/svc"
	"github.com/askxuan/master-service/rpc/internal/server"
	"github.com/askxuan/master-service/rpc/master"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
)

func MustStartMasterRpcServer(c config.Config, svcCtx *svc.ServiceContext) *zrpc.RpcServer {
	s := zrpc.MustNewServer(c.MasterRpc, func(gs *grpc.Server) {
		master.RegisterMasterBookingServiceServer(gs, server.NewMasterBookingServer(svcCtx))
	})
	go s.Start()
	fmt.Printf("启动 master-service gRPC，监听 %s\n", c.MasterRpc.ListenOn)
	return s
}
