package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/askxuan/common"
)

// 网关透传给下游服务的请求头
const (
	HeaderUserID   = "X-User-Id"
	HeaderUserMobile = "X-User-Mobile"
	HeaderUserType = "X-User-Type"
	HeaderRoles    = "X-User-Roles"
	HeaderClientID = "X-Client-Id"
	HeaderTempleID = "X-Temple-Id"
	HeaderMasterID = "X-Master-Id"
)

// Auth 全局 JWT 鉴权中间件
// - 白名单路径直接放行（前缀匹配：GET 请求匹配 path == prefix 或 path 以 prefix+"/" 开头）
// - 校验 Authorization: Bearer <token>，解析后将用户信息注入请求头透传下游
// - /api/v1/admin/* 路径额外校验管理台角色
func Auth(secret string, noAuthPaths []string) func(http.Handler) http.Handler {
	whitelist := noAuthPaths

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 白名单放行：GET 请求前缀匹配（支持 /temples 和 /temples/T001 等公开浏览接口）
			if r.Method == http.MethodGet {
				for _, p := range whitelist {
					if r.URL.Path == p || strings.HasPrefix(r.URL.Path, p+"/") {
						next.ServeHTTP(w, r)
						return
					}
				}
			} else {
				// 非 GET 请求精确匹配白名单
				for _, p := range whitelist {
					if r.URL.Path == p {
						next.ServeHTTP(w, r)
						return
					}
				}
			}
			// 解析 token
			auth := r.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "Bearer ") {
				common.JsonError(w, common.ErrUnauthorized)
				return
			}
			tokenStr := strings.TrimPrefix(auth, "Bearer ")
			claims, err := common.ParseToken(secret, tokenStr)
			if err != nil {
				common.JsonError(w, common.ErrTokenInvalid)
				return
			}
			if claims.IsRefreshToken() {
				common.JsonError(w, common.ErrTokenInvalid)
				return
			}

			// 管理台/工作台路径角色校验：/api/v1/admin/* 需要管理台角色，法师工作台也使用该前缀
			if strings.HasPrefix(r.URL.Path, "/api/v1/admin/") {
				if !claims.IsAdmin() && !claims.HasRole("master") {
					common.JsonError(w, common.ErrRoleForbidden)
					return
				}
			}

			// 注入用户信息到请求头，透传给下游服务
			r.Header.Set(HeaderUserID, strconv.FormatInt(claims.UserId, 10))
			if claims.Mobile != "" {
				r.Header.Set(HeaderUserMobile, claims.Mobile)
			}
			if claims.UserType != "" {
				r.Header.Set(HeaderUserType, claims.UserType)
			}
			if len(claims.Roles) > 0 {
				r.Header.Set(HeaderRoles, strings.Join(claims.Roles, ","))
			}
			if claims.ClientID != "" {
				r.Header.Set(HeaderClientID, claims.ClientID)
			}
			if claims.TempleID > 0 {
				r.Header.Set(HeaderTempleID, strconv.FormatInt(claims.TempleID, 10))
			}
			if claims.MasterID > 0 {
				r.Header.Set(HeaderMasterID, strconv.FormatInt(claims.MasterID, 10))
			}
			next.ServeHTTP(w, r)
		})
	}
}
