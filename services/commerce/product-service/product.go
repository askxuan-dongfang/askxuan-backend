package main

import (
	"flag"
	"fmt"

	"github.com/askxuan/product-service/internal/config"
	"github.com/askxuan/product-service/internal/handler"
	"github.com/askxuan/product-service/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/product.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	svcCtx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, svcCtx)

	fmt.Printf("启动 product-service，监听 %s:%d\n", c.Host, c.Port)
	server.Start()
}
