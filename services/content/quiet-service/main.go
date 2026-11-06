package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	store := memoryStorage()
	if os.Getenv("QUIET_MYSQL_ADDR") != "" || os.Getenv("QUIET_REDIS_ADDR") != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		var err error
		prefix := os.Getenv("QUIET_REDIS_PREFIX")
		if prefix == "" {
			prefix = "quiet-park-local:"
		}
		store, err = persistentStorage(ctx, os.Getenv("QUIET_MYSQL_ADDR"), os.Getenv("QUIET_MYSQL_PASSWORD"), os.Getenv("QUIET_REDIS_ADDR"), os.Getenv("QUIET_REDIS_PASSWORD"), prefix)
		cancel()
		if err != nil {
			log.Fatal(err)
		}
	}
	defer store.close()
	service, err := newApp(store, os.Getenv("QUIET_CSRF_KEY"))
	if err != nil {
		log.Fatal("无法初始化静心访客服务")
	}
	defer service.close()
	service.origins, err = parseOrigins(os.Getenv("QUIET_PUBLIC_ORIGINS"))
	if err != nil {
		log.Fatal(err)
	}
	service.authURL, err = authEndpoint(os.Getenv("QUIET_AUTH_URL"))
	if err != nil {
		log.Fatal(err)
	}
	address := os.Getenv("QUIET_LISTEN")
	if address == "" {
		address = "127.0.0.1:8096"
	}
	if address != "127.0.0.1:8096" && address != "0.0.0.0:8096" {
		log.Fatal("只允许本地默认地址或隔离容器内部地址")
	}
	server := &http.Server{Addr: address, Handler: service.handler(), ReadHeaderTimeout: 5 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("静心园区服务：http://%s（每房20人代码上限，独立持久化=%t，账号历史鉴权配置=%t，房间仍为访客）", server.Addr, store.db != nil, service.authURL != "")
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
