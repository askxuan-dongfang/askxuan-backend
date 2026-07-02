package middleware

import (
	"net/http"

	"github.com/askxuan/common"
)

// AdminAuthConfig 管理台角色鉴权中间件配置
type AdminAuthConfig struct {
	// AllowedRoles 允许访问的角色列表
	// 为空时允许所有管理台角色（temple_admin/master/shop_admin/platform_super/platform_service）
	AllowedRoles []string
}

// AdminAuth 管理台角色鉴权中间件（http.Handler 风格）
// 必须在 Auth 中间件之后使用，从 context 中读取已解析的角色信息
func (c *AdminAuthConfig) AdminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roles := RolesFromCtx(r.Context())
		if len(roles) == 0 {
			common.JsonError(w, common.ErrForbidden)
			return
		}
		if !c.hasAllowedRole(roles) {
			common.JsonError(w, common.ErrRoleForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// AdminAuthFunc go-zero rest 风格管理台角色鉴权中间件
func (c *AdminAuthConfig) AdminAuthFunc(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		roles := RolesFromCtx(r.Context())
		if len(roles) == 0 {
			common.JsonError(w, common.ErrForbidden)
			return
		}
		if !c.hasAllowedRole(roles) {
			common.JsonError(w, common.ErrRoleForbidden)
			return
		}
		next(w, r)
	}
}

// hasAllowedRole 检查角色是否在允许列表中
func (c *AdminAuthConfig) hasAllowedRole(roles []string) bool {
	// 未配置 AllowedRoles 时，允许所有管理台角色
	if len(c.AllowedRoles) == 0 {
		for _, role := range roles {
			if role == "temple_admin" || role == "master" ||
				role == "shop_admin" || role == "platform_super" || role == "platform_service" {
				return true
			}
		}
		return false
	}
	// 配置了 AllowedRoles 时，精确匹配
	for _, allowed := range c.AllowedRoles {
		for _, role := range roles {
			if role == allowed {
				return true
			}
		}
	}
	return false
}
