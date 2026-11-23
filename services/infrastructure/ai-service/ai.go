package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/askxuan/ai-service/internal/catalogjob"
	"github.com/askxuan/ai-service/internal/config"
	"github.com/askxuan/ai-service/internal/handler"
	"github.com/askxuan/ai-service/internal/provider"
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
	go (catalogjob.Store{DB: svcCtx.DB}).Run(ctx, func(ctx context.Context, task catalogjob.TranslationTask, source map[string]string) (map[string]string, error) {
		runtime := svcCtx.Settings.Snapshot()
		route := runtime.CatalogTranslation
		// A dedicated OpenCode Go connection receives only operator-authored copy.
		if task.Kind == "diy-design" && route.Provider != nil {
			route = provider.RegisteredRoute{}
		}
		if route.Provider == nil {
			model, err := runtime.Models.Select(ctx, route.Model, false)
			if err != nil {
				return nil, err
			}
			route.Provider, route.Model = runtime.Provider, model
		}
		if task.Kind == "diy-design" {
			p, req, err := provider.ResolveProvider(ctx, route.Provider, provider.Request{Model: route.Model})
			if err != nil {
				return nil, err
			}
			if p.Name() == provider.OpenCodeGo {
				p = provider.PrimaryProvider(runtime.Provider)
				req.Model = p.Model()
			}
			route.Provider, route.Model = p, req.Model
		}
		budget := runtime.Config.MaxOutputTokens
		if limit := route.Option.MaxOutputTokens; limit > 0 && (budget <= 0 || limit < budget) {
			budget = limit
		}
		return catalogjob.Translate(ctx, route.Provider, route.Model, task.SessionID, budget, source)
	})

	fmt.Printf("启动 ai-service，监听 %s:%d\n", c.Host, c.Port)
	server.Start()
}
