package middleware

import (
	"net/http"

	"github.com/askxuan/common"
	"github.com/askxuan/common/adminpolicy"
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
		if !c.hasAllowedRole(roles) && !delegatedRequestAllowed(r) {
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
		if !c.hasAllowedRole(roles) && !delegatedRequestAllowed(r) {
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
			if adminpolicy.IsTempleRole(role) || role == "master" ||
				role == "shop_admin" || role == "platform_super" || role == "platform_service" {
				return true
			}
		}
		return false
	}
	// 配置了 AllowedRoles 时，精确匹配
	for _, allowed := range c.AllowedRoles {
		for _, role := range roles {
			if role == allowed || (allowed == "temple_admin" && adminpolicy.IsTempleStaff(role)) {
				return true
			}
		}
	}
	return false
}

// Delegation applies only to platform APIs explicitly catalogued for this method.
// A token for a provider/customer route cannot use this to change its identity domain.
func delegatedRequestAllowed(r *http.Request) bool {
	c := AdminClaimsFromCtx(r.Context())
	if c == nil || c.UserType != "admin" || c.ClientID != "platform-admin" || c.AuthzScope != "platform" || len(c.Roles) != 1 || adminpolicy.Builtin(c.Roles[0]) {
		return false
	}
	return adminpolicy.Allowed(r.Method, r.URL.Path, c.Permissions)
}
