// Package middleware 提供各微服务复用的 HTTP 中间件
package middleware

import "net/http"

// CorsHeaders 设置跨域响应头，允许前端联调
func CorsHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Requested-With")
	w.Header().Set("Access-Control-Max-Age", "3600")
}

// Cors CORS 中间件，处理预检请求并放行后续
func Cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		CorsHeaders(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// CorsFunc go-zero rest 风格的 CORS 中间件（HandlerFunc 签名）
func CorsFunc(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		CorsHeaders(w)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}
