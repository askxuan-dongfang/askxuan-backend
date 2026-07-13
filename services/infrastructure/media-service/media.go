package main

import (
	"flag"
	"fmt"

	"github.com/askxuan/media-service/internal/config"
	"github.com/askxuan/media-service/internal/handler"
	"github.com/askxuan/media-service/internal/svc"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/media.yaml", "the config file")

func main() {
	flag.Parse()
	var c config.Config
	conf.MustLoad(*configFile, &c)
	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()
	ctx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, ctx)
	fmt.Printf("启动 media-service，监听 %s:%d\n", c.Host, c.Port)
	server.Start()
}
