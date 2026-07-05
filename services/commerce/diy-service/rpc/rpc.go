package rpc

import (
	"fmt"

	"github.com/askxuan/diy-service/internal/config"
	"github.com/askxuan/diy-service/internal/svc"
	"github.com/askxuan/diy-service/rpc/diy"
	"github.com/askxuan/diy-service/rpc/internal/server"

	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
)

// MustStartDiyRpcServer 启动 diy-service 的 gRPC server（供 master/temple 通过 zrpc 调用）
// 使用主 ServiceContext 复用 DB 连接和 BlessingTaskModel
// 返回 *zrpc.RpcServer 供调用方 defer Stop
func MustStartDiyRpcServer(c config.Config, svcCtx *svc.ServiceContext) *zrpc.RpcServer {
	rpcServer := zrpc.MustNewServer(c.DiyRpc, func(grpcServer *grpc.Server) {
		diy.RegisterDiyServiceServer(grpcServer, server.NewDiyServer(svcCtx))
	})
	go rpcServer.Start()
	fmt.Printf("启动 diy-service gRPC，监听 %s\n", c.DiyRpc.ListenOn)
	return rpcServer
}
