package logic

import (
	"context"
	"strings"
	"time"

	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/auth-service/internal/types"
	"github.com/askxuan/common"
	"github.com/askxuan/common/identity"
	"github.com/golang-jwt/jwt/v5"

	"github.com/zeromicro/go-zero/core/logx"
)

// LogoutLogic 登出逻辑
type LogoutLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogoutLogic {
	return &LogoutLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Logout 将 access token 写入 Redis 黑名单，TTL = 剩余有效期
func (l *LogoutLogic) Logout(req *types.LogoutReq) (*types.LogoutResp, error) {
	tokenStr := strings.TrimSpace(req.AccessToken)
	if tokenStr == "" {
		// 从请求头取
		if auth := l.ctx.Value("authorization"); auth != nil {
			if s, ok := auth.(string); ok {
				tokenStr = strings.TrimPrefix(s, "Bearer ")
			}
		}
	}
	if tokenStr == "" {
		// 无 token 视为已登出
		return &types.LogoutResp{Success: true}, nil
	}

	claims, err := revocationClaims(l.svcCtx.Config.Auth.AccessSecret, tokenStr)
	if err == nil && claims != nil {
		if l.svcCtx.SessionRedis != nil {
			if e := identity.RevokeSession(l.ctx, l.svcCtx.SessionRedis, claims.SessionID); e != nil {
				return nil, common.ErrSystem
			}
		}
		// 计算剩余有效期，作为黑名单 TTL
		ttl := time.Until(claims.ExpiresAt.Time)
		if ttl > 0 && l.svcCtx.Redis != nil {
			blackKey := "jwt:blacklist:" + tokenStr
			_ = l.svcCtx.Redis.Setex(blackKey, "1", int(ttl.Seconds()))
		}
	}

	return &types.LogoutResp{Success: true}, nil
}

// Expired, correctly signed tokens may revoke their own session, but can never
// authenticate a request. This parser is deliberately private to logout.
func revocationClaims(secret, value string) (*common.CustomClaims, error) {
	claims := &common.CustomClaims{}
	_, err := jwt.ParseWithClaims(value, claims, func(_ *jwt.Token) (interface{}, error) { return []byte(secret), nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithoutClaimsValidation())
	if err != nil {
		return nil, err
	}
	if claims.ExpiresAt == nil {
		return nil, common.ErrTokenInvalid
	}
	return claims, nil
}
