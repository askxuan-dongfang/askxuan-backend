package diy

import "github.com/zeromicro/go-zero/zrpc"

// NewDiyService zrpc 客户端构造（供 master-service / temple-service 通过 etcd 发现调用）
// 参数 zrpcClient 由 zrpc.MustNewClient(c.DiyRpc) 创建
func NewDiyService(zrpcClient zrpc.Client) DiyServiceClient {
	return NewDiyServiceClient(zrpcClient.Conn())
}
