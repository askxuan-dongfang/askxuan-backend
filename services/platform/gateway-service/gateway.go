package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/askxuan/gateway-service/internal/config"
	"github.com/askxuan/gateway-service/internal/discovery"
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

	// 提取所有需要发现的服务名（对应各下游服务 yaml 中 Etcd.Key 的值）
	// 仅用于动态发现；当前 rest 服务未注册到 etcd，主要走 Upstream.Target 静态路由。
	serviceNames := make([]string, 0, len(c.Upstreams))
	seen := make(map[string]bool, len(c.Upstreams))
	for _, u := range c.Upstreams {
		if u.ServiceName == "" || seen[u.ServiceName] {
			continue
		}
		seen[u.ServiceName] = true
		serviceNames = append(serviceNames, u.ServiceName)
	}

	// 初始化 etcd 服务发现（失败时 disc=nil，proxy 自动回退到静态 Target）
	// 仅当配置了 Etcd.Hosts 且有服务名需要发现时才尝试连接，避免无谓的 etcd 依赖。
	var disc *discovery.Discovery
	if len(c.Etcd.Hosts) > 0 && len(serviceNames) > 0 {
		d, err := discovery.New(c.Etcd.Hosts, serviceNames)
		if err != nil {
			logx.Errorf("初始化服务发现失败，将使用静态 Target 路由: %v", err)
		} else {
			disc = d
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if disc != nil {
		if err := disc.Watch(ctx); err != nil {
			logx.Errorf("启动服务发现 Watch 失败（后台将持续重试）: %v", err)
		}
		logx.Infof("服务发现已启动，etcd=%v", c.Etcd.Hosts)
	} else {
		logx.Infof("服务发现未启用，使用 Upstream.Target 静态路由")
	}

	// 构建反向代理 Handler
	proxyHandler := proxy.New(c.Upstreams, disc)

	// 本地健康检查直接在 gateway 返回，避免依赖下游服务或反代配置。
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":{"service":"gateway"}}`))
	})
	mux.Handle("/", proxyHandler)

	// 中间件链：CORS → 全局 JWT 鉴权（白名单 + 透传 X-User-Id）→ 反向代理
	var handler http.Handler = mux
	handler = middleware.Auth(c.Auth.AccessSecret, c.NoAuthPaths)(handler)
	handler = middleware.Cors(handler)

	addr := fmt.Sprintf("%s:%d", c.Host, c.Port)
	server := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	// 优雅关闭：收到信号后停止接收新连接并释放 etcd 资源
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		logx.Infof("收到退出信号，开始优雅关闭")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	logx.Infof("启动 gateway，监听 %s，路由 %d 个前缀", addr, len(c.Upstreams))
	fmt.Printf("启动 gateway，监听 %s，路由 %d 个前缀\n", addr, len(c.Upstreams))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logx.Errorf("gateway 启动失败: %v", err)
		panic(err)
	}

	// 服务停止后释放服务发现资源
	cancel()
	if disc != nil {
		_ = disc.Close()
	}
}
