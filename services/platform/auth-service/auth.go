package main

import (
	"flag"
	"fmt"

	"github.com/askxuan/auth-service/internal/config"
	"github.com/askxuan/auth-service/internal/handler"
	"github.com/askxuan/auth-service/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/auth.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	// 关闭 etcd 服务注册（MVP-1 本地联调，避免强依赖 etcd 可用性）
	// 生产环境应保留 Telemetry 配置以注册到 etcd 供网关发现
	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	svcCtx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, svcCtx)

	fmt.Printf("启动 auth-service，监听 %s:%d\n", c.Host, c.Port)
	server.Start()
}
