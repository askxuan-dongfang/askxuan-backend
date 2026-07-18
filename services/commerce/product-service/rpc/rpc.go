package rpc

import (
	"fmt"

	"github.com/askxuan/common/rpc/catalog"
	"github.com/askxuan/product-service/internal/config"
	"github.com/askxuan/product-service/internal/svc"
	"github.com/askxuan/product-service/rpc/internal/server"
	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
)

func MustStartProductRpcServer(c config.Config, svcCtx *svc.ServiceContext) *zrpc.RpcServer {
	s := zrpc.MustNewServer(c.ProductRpc, func(gs *grpc.Server) {
		catalog.RegisterCatalogServiceServer(gs, server.NewCatalogServer(svcCtx))
	})
	go s.Start()
	fmt.Printf("启动 product-service gRPC，监听 %s\n", c.ProductRpc.ListenOn)
	return s
}
