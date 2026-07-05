package proxy

import (
	"context"
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/askxuan/gateway-service/internal/config"
	"github.com/askxuan/gateway-service/internal/discovery"
	"github.com/zeromicro/go-zero/core/logx"
)

// targetAddrKey 用于在 request context 中传递动态选中的下游实例地址
type targetAddrKey struct{}

// New 构造网关反向代理 Handler
// 路由匹配按前缀长度降序（最长前缀优先）。
// 每条 Upstream 的目标地址选择优先级：
//  1. 配置了 ServiceName 且 discovery 找到实例 → 用动态发现的实例（轮询）
//  2. 配置了 Target（静态地址） → 直接用 Target
//  3. 都没有 → 返回 502
func New(upstreams []config.Upstream, disc *discovery.Discovery) http.Handler {
	// 构建前缀匹配表，按前缀长度降序排列以保证最长前缀优先匹配
	routes := make([]route, 0, len(upstreams))
	for _, u := range upstreams {
		routes = append(routes, route{
			prefix:      u.Prefix,
			target:      u.Target,
			serviceName: u.ServiceName,
		})
	}
	sortRoutes(routes)

	// 共用一个 ReverseProxy，目标地址在 Director 中通过 context 动态设置
	proxy := &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			addr, _ := r.Context().Value(targetAddrKey{}).(string)
			r.URL.Scheme = "http"
			r.URL.Host = addr
			// 透传原始 Host 与路径，去除下游不需要的头
			r.Header.Set("X-Forwarded-Host", r.Host)
			if _, ok := r.Header["User-Agent"]; !ok {
				r.Header.Set("User-Agent", "")
			}
		},
		// 自定义错误处理，避免网关直接 502 暴露内部错误
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			logx.Errorf("代理请求失败 path=%s: %v", r.URL.Path, err)
			writeBadGateway(w, "下游服务不可用")
		},
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, rt := range routes {
			if !strings.HasPrefix(r.URL.Path, rt.prefix) {
				continue
			}
			addr, ok := rt.pick(disc)
			if !ok {
				logx.Errorf("无可用目标 prefix=%s service=%s target=%s path=%s",
					rt.prefix, rt.serviceName, rt.target, r.URL.Path)
				writeBadGateway(w, "下游服务暂不可用")
				return
			}
			ctx := context.WithValue(r.Context(), targetAddrKey{}, addr)
			proxy.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		// 无匹配路由
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"code":4004,"message":"路由不存在"}`))
	})
}

// pick 按优先级选择目标地址：动态发现 → 静态 Target
func (rt route) pick(disc *discovery.Discovery) (string, bool) {
	// 1) 优先动态发现
	if rt.serviceName != "" && disc != nil {
		if ins, ok := disc.Pick(rt.serviceName); ok {
			return ins.Addr, true
		}
	}
	// 2) 回退静态 Target
	if rt.target != "" {
		return rt.target, true
	}
	// 3) 都没有
	return "", false
}

func writeBadGateway(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusBadGateway)
	_, _ = w.Write([]byte(`{"code":5000,"message":"` + msg + `"}`))
}

type route struct {
	prefix      string
	target      string
	serviceName string
}

func sortRoutes(routes []route) {
	// 简单插入排序，按 prefix 长度降序
	for i := 1; i < len(routes); i++ {
		for j := i; j > 0 && len(routes[j].prefix) > len(routes[j-1].prefix); j-- {
			routes[j], routes[j-1] = routes[j-1], routes[j]
		}
	}
}
