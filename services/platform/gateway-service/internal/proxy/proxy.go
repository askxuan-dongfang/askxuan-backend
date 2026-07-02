package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/askxuan/gateway-service/internal/config"
	"github.com/zeromicro/go-zero/core/logx"
)

// New 构造网关反向代理 Handler
// 根据请求路径前缀匹配 Upstreams，转发到对应 Target
func New(upstreams []config.Upstream) http.Handler {
	// 构建前缀匹配表，按前缀长度降序排列以保证最长前缀优先匹配
	routes := make([]route, 0, len(upstreams))
	for _, u := range upstreams {
		target, err := url.Parse("http://" + u.Target)
		if err != nil {
			logx.Errorf("解析上游地址失败 prefix=%s target=%s: %v", u.Prefix, u.Target, err)
			continue
		}
		proxy := httputil.NewSingleHostReverseProxy(target)
		// 自定义错误处理，避免网关直接 502 暴露内部错误
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			logx.Errorf("代理请求失败 prefix=%s path=%s target=%s: %v", u.Prefix, r.URL.Path, u.Target, err)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"code":5000,"message":"下游服务不可用"}`))
		}
		routes = append(routes, route{prefix: u.Prefix, proxy: proxy})
	}
	// 按前缀长度降序，保证 /api/v1/auth 比 /api/v1 优先匹配（当前前缀均同级，留作扩展）
	sortRoutes(routes)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, rt := range routes {
			if strings.HasPrefix(r.URL.Path, rt.prefix) {
				// 透传原始 Host 与路径，去除下游不需要的头
				r.Header.Set("X-Forwarded-Host", r.Host)
				rt.proxy.ServeHTTP(w, r)
				return
			}
		}
		// 无匹配路由
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":4004,"message":"路由不存在"}`))
	})
}

type route struct {
	prefix string
	proxy  *httputil.ReverseProxy
}

func sortRoutes(routes []route) {
	// 简单插入排序，按 prefix 长度降序
	for i := 1; i < len(routes); i++ {
		for j := i; j > 0 && len(routes[j].prefix) > len(routes[j-1].prefix); j-- {
			routes[j], routes[j-1] = routes[j-1], routes[j]
		}
	}
}
