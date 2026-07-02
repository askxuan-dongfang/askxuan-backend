package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/askxuan/common"
)

// ctxKey context key 类型，避免键冲突
type ctxKey string

// 预定义 context key
const (
	CtxKeyUserID   ctxKey = "userId"
	CtxKeyMobile   ctxKey = "mobile"
	CtxKeyUserType ctxKey = "userType"
	CtxKeyRoles    ctxKey = "roles"
	CtxKeyClientID ctxKey = "clientId"
	CtxKeyTempleID ctxKey = "templeId"
	CtxKeyMasterID ctxKey = "masterId"
)

// AuthConfig 鉴权中间件配置
type AuthConfig struct {
	Secret      string   // JWT 签名密钥
	NoAuthPaths []string // 不需要鉴权的白名单路径（精确匹配，如 /api/v1/auth/login）
}

// IsWhitelisted 判断路径是否在白名单中
func (c *AuthConfig) IsWhitelisted(path string) bool {
	for _, p := range c.NoAuthPaths {
		if p == path {
			return true
		}
	}
	return false
}

// Auth JWT 鉴权中间件（http.Handler 风格）
// 校验 Authorization: Bearer <token>，并将 userId 注入 context
func (c *AuthConfig) Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 白名单放行
		if c.IsWhitelisted(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		if err := c.doAuth(w, r); err != nil {
			common.JsonError(w, err)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// AuthFunc go-zero rest 风格鉴权中间件（HandlerFunc 签名）
func (c *AuthConfig) AuthFunc(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if c.IsWhitelisted(r.URL.Path) {
			next(w, r)
			return
		}
		if err := c.doAuth(w, r); err != nil {
			common.JsonError(w, err)
			return
		}
		next(w, r)
	}
}

// doAuth 实际执行 token 校验与 context 注入
func (c *AuthConfig) doAuth(_ http.ResponseWriter, r *http.Request) error {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return common.ErrUnauthorized
	}
	tokenStr := strings.TrimPrefix(auth, "Bearer ")
	claims, err := common.ParseToken(c.Secret, tokenStr)
	if err != nil {
		return common.ErrTokenInvalid
	}
	if claims.IsRefreshToken() {
		// refresh token 不能用于接口鉴权
		return common.ErrTokenInvalid
	}
	// 注入用户信息到 context
	ctx := r.Context()
	ctx = context.WithValue(ctx, CtxKeyUserID, claims.UserId)
	ctx = context.WithValue(ctx, CtxKeyMobile, claims.Mobile)
	ctx = context.WithValue(ctx, CtxKeyUserType, claims.UserType)
	ctx = context.WithValue(ctx, CtxKeyRoles, claims.Roles)
	ctx = context.WithValue(ctx, CtxKeyClientID, claims.ClientID)
	ctx = context.WithValue(ctx, CtxKeyTempleID, claims.TempleID)
	ctx = context.WithValue(ctx, CtxKeyMasterID, claims.MasterID)
	*r = *r.WithContext(ctx)
	return nil
}

// UserIDFromCtx 从 context 取出 userId（未登录返回 0）
func UserIDFromCtx(ctx context.Context) int64 {
	if v, ok := ctx.Value(CtxKeyUserID).(int64); ok {
		return v
	}
	return 0
}

// MobileFromCtx 从 context 取出 mobile
func MobileFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(CtxKeyMobile).(string); ok {
		return v
	}
	return ""
}

// RolesFromCtx 从 context 取出角色列表
func RolesFromCtx(ctx context.Context) []string {
	if v, ok := ctx.Value(CtxKeyRoles).([]string); ok {
		return v
	}
	return nil
}

// ClientIDFromCtx 从 context 取出 clientId
func ClientIDFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(CtxKeyClientID).(string); ok {
		return v
	}
	return ""
}

// TempleIDFromCtx 从 context 取出 templeId
func TempleIDFromCtx(ctx context.Context) int64 {
	if v, ok := ctx.Value(CtxKeyTempleID).(int64); ok {
		return v
	}
	return 0
}

// MasterIDFromCtx 从 context 取出 masterId
func MasterIDFromCtx(ctx context.Context) int64 {
	if v, ok := ctx.Value(CtxKeyMasterID).(int64); ok {
		return v
	}
	return 0
}
