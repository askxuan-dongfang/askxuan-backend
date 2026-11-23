package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/askxuan/ai-service/internal/catalogjob"
	"github.com/askxuan/ai-service/internal/config"
	"github.com/askxuan/ai-service/internal/handler"
	"github.com/askxuan/ai-service/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/ai.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)
	// Credential-bearing admin requests use metadata-only logging.
	c.Middlewares.Log = false

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()
	server.Use(handler.SafeRequestLog)

	svcCtx := svc.NewServiceContext(c)
	handler.RegisterHandlers(server, svcCtx)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go (catalogjob.Store{DB: svcCtx.DB}).Run(ctx, func(ctx context.Context, source map[string]string) (map[string]string, error) {
		runtime := svcCtx.Runtime()
		return catalogjob.Translate(ctx, runtime.Provider, runtime.Models, runtime.AIConfig.MaxOutputTokens, source)
	})

	fmt.Printf("启动 ai-service，监听 %s:%d\n", c.Host, c.Port)
	server.Start()
}
