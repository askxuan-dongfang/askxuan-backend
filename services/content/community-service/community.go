package main

import (
	"flag"
	"fmt"

	"github.com/askxuan/community-service/internal/config"
	"github.com/askxuan/community-service/internal/handler"
	"github.com/askxuan/community-service/internal/svc"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/community.yaml", "the config file")

func main() {
	flag.Parse()
	var c config.Config
	conf.MustLoad(*configFile, &c)
	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()
	handler.RegisterHandlers(server, svc.NewServiceContext(c))
	fmt.Printf("启动 community-service，监听 %s:%d\n", c.Host, c.Port)
	server.Start()
}
