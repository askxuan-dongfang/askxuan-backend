package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/askxuan/gateway-service/internal/config"
	"github.com/askxuan/gateway-service/internal/middleware"
	"github.com/askxuan/gateway-service/internal/proxy"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/logx"
)

var configFile = flag.String("f", "etc/gateway.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	// 构建反向代理 Handler
	proxyHandler := proxy.New(c.Upstreams)

	// 中间件链：CORS → 全局 JWT 鉴权（白名单 + 透传 X-User-Id）→ 反向代理
	var handler http.Handler = proxyHandler
	handler = middleware.Auth(c.Auth.AccessSecret, c.NoAuthPaths)(handler)
	handler = middleware.Cors(handler)

	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	logx.Infof("启动 gateway，监听 %s，路由 %d 个前缀", addr, len(c.Upstreams))
	fmt.Printf("启动 gateway，监听 %s，路由 %d 个前缀\n", addr, len(c.Upstreams))
	if err := server.ListenAndServe(); err != nil {
		logx.Errorf("gateway 启动失败: %v", err)
		panic(err)
	}
}
