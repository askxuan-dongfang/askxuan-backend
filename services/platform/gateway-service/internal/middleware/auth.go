package middleware

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/askxuan/common"
)

// 网关透传给下游服务的请求头
const (
	HeaderUserID     = "X-User-Id"
	HeaderUserMobile = "X-User-Mobile"
	HeaderUserType   = "X-User-Type"
	HeaderRoles      = "X-User-Roles"
	HeaderClientID   = "X-Client-Id"
	HeaderTempleID   = "X-Temple-Id"
	HeaderTempleCode = "X-Temple-Code"
	HeaderMasterID   = "X-Master-Id"
)

type adminRoleRule struct {
	prefix string
	roles  []string
}

// Order matters: more specific routes must precede their parent prefixes.
var adminRoleRules = []adminRoleRule{
	{prefix: "/api/v1/admin/finance/withdrawals/apply", roles: []string{"master"}},
	{prefix: "/api/v1/admin/platform/", roles: []string{"platform_super"}},
	{prefix: "/api/v1/admin/masters/", roles: []string{"master"}},
	{prefix: "/api/v1/admin/temples/", roles: []string{"temple_admin"}},
	{prefix: "/api/v1/admin/reviews", roles: []string{"temple_admin", "master"}},
	{prefix: "/api/v1/admin/bookings", roles: []string{"temple_admin"}},
	{prefix: "/api/v1/admin/points", roles: []string{"shop_admin"}},
	{prefix: "/api/v1/admin/products", roles: []string{"shop_admin"}},
	{prefix: "/api/v1/admin/diy", roles: []string{"shop_admin"}},
	{prefix: "/api/v1/admin/orders", roles: []string{"shop_admin"}},
	{prefix: "/api/v1/admin/logistics", roles: []string{"shop_admin"}},
	{prefix: "/api/v1/admin/files", roles: []string{"platform_super"}},
	{prefix: "/api/v1/admin/auth", roles: []string{"platform_super"}},
	{prefix: "/api/v1/admin/users", roles: []string{"platform_super"}},
	{prefix: "/api/v1/admin/finance", roles: []string{"platform_super"}},
	{prefix: "/api/v1/admin/audit", roles: []string{"platform_super"}},
	{prefix: "/api/v1/admin/marketing", roles: []string{"platform_super"}},
	{prefix: "/api/v1/admin/announcements", roles: []string{"platform_super"}},
}

func roleAllowedForAdminPath(path string, claims *common.CustomClaims) bool {
	// Master inbox routes are more specific than platform message management.
	if path == "/api/v1/admin/messages/master" || strings.HasPrefix(path, "/api/v1/admin/messages/master/") {
		return claims.UserType == "admin" && claims.HasRole("master") && claims.MasterID > 0
	}
	if path == "/api/v1/admin/messages" || strings.HasPrefix(path, "/api/v1/admin/messages/") ||
		path == "/api/v1/admin/announcements" || strings.HasPrefix(path, "/api/v1/admin/announcements/") {
		// Internal notifications use MQ, not these human management endpoints.
		return claims.UserType == "admin" && claims.HasRole("platform_super")
	}
	// Provider credentials are restricted to human platform super administrators.
	if path == "/api/v1/ai/admin" || strings.HasPrefix(path, "/api/v1/ai/admin/") {
		return claims.UserType == "admin" && claims.HasRole("platform_super")
	}
	// Internal service tokens retain access to administrative integration APIs.
	if claims.UserType == "service" && claims.HasRole("platform_service") {
		return true
	}
	// Roles alone never establish an administrator identity, including legacy claims.
	if claims.UserType != "admin" {
		return false
	}
	for _, rule := range adminRoleRules {
		if path == strings.TrimSuffix(rule.prefix, "/") || strings.HasPrefix(path, rule.prefix) {
			if claims.HasRole("platform_super") {
				return true
			}
			for _, role := range rule.roles {
				if claims.HasRole(role) {
					return true
				}
			}
			return false
		}
	}
	return claims.IsAdmin() || claims.HasRole("master")
}

// Auth 全局 JWT 鉴权中间件
// - 公开 GET 允许访客访问；携带 token 时验证并透传身份（GET 白名单支持子路径）
// - 校验 Authorization: Bearer <token>，解析后将用户信息注入请求头透传下游
// - /api/v1/admin/* 路径额外校验管理台角色
func Auth(secret string, noAuthPaths []string, sessionChecks ...func(context.Context, *common.CustomClaims) error) func(http.Handler) http.Handler {
	whitelist := append(append([]string{}, noAuthPaths...), "/api/v1/auth/options", "/api/v1/auth/captcha", "/api/v1/auth/email/code", "/api/v1/auth/email/register", "/api/v1/auth/password/reset", "/api/v1/auth/work/register", "/api/v1/auth/work/activate")

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only verified JWT claims may establish a downstream identity, including
			// on public routes. Never retain client-supplied identity headers.
			for _, header := range []string{HeaderUserID, HeaderUserMobile, HeaderUserType, HeaderRoles, HeaderClientID, HeaderTempleID, HeaderTempleCode, HeaderMasterID} {
				r.Header.Del(header)
			}
			publicRead := false
			if r.Method == http.MethodGet {
				publicRead = r.URL.Path == "/api/v1/marketing/activities" || strings.HasPrefix(r.URL.Path, "/api/v1/marketing/activities/")
				for _, p := range whitelist {
					if r.URL.Path == p || strings.HasPrefix(r.URL.Path, p+"/") {
						publicRead = true
						break
					}
				}
			} else {
				// Login/refresh and other explicitly public writes retain their
				// own credential validation, even if a stale bearer was sent.
				for _, p := range whitelist {
					if r.URL.Path == p {
						next.ServeHTTP(w, r)
						return
					}
				}
			}
			if publicRead && r.Header.Get("Authorization") == "" {
				next.ServeHTTP(w, r)
				return
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

			if claims.UserType != "service" {
				for _, check := range sessionChecks {
					if e := check(r.Context(), claims); e != nil {
						common.JsonError(w, common.ErrTokenInvalid)
						return
					}
				}
			}
			if (claims.HasRole("master_applicant") || claims.HasRole("temple_applicant")) && !strings.HasPrefix(r.URL.Path, "/api/v1/auth/onboarding") && r.URL.Path != "/api/v1/auth/logout" {
				common.JsonError(w, common.ErrRoleForbidden)
				return
			}
			// 管理台/工作台路径角色校验：/api/v1/admin/* 需要管理台角色，法师工作台也使用该前缀
			if strings.HasPrefix(r.URL.Path, "/api/v1/admin/") || r.URL.Path == "/api/v1/ai/admin" || strings.HasPrefix(r.URL.Path, "/api/v1/ai/admin/") {
				if !roleAllowedForAdminPath(r.URL.Path, claims) {
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
			if claims.TempleCode != "" {
				r.Header.Set(HeaderTempleCode, claims.TempleCode)
			}
			if claims.MasterID > 0 {
				r.Header.Set(HeaderMasterID, strconv.FormatInt(claims.MasterID, 10))
			}
			next.ServeHTTP(w, r)
		})
	}
}
