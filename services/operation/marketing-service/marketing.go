package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/zeromicro/go-zero/core/logx"
	"time"

	"github.com/askxuan/marketing-service/internal/config"
	"github.com/askxuan/marketing-service/internal/handler"
	"github.com/askxuan/marketing-service/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/marketing.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	svcCtx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, svcCtx)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			runCtx, stop := context.WithTimeout(ctx, 2*time.Minute)
			if err := svcCtx.Rewards.DrawDue(runCtx); err != nil {
				logx.Errorf("reward automatic draw: %v", err)
			}
			stop()
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()

	fmt.Printf("启动 marketing-service，监听 %s:%d\n", c.Host, c.Port)
	server.Start()
}
