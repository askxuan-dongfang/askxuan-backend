package logic

import (
	"context"
	"strings"
	"time"

	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/auth-service/internal/types"
	"github.com/askxuan/common"

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

	claims, err := common.ParseToken(l.svcCtx.Config.Auth.AccessSecret, tokenStr)
	if err == nil && claims != nil {
		// 计算剩余有效期，作为黑名单 TTL
		ttl := time.Until(claims.ExpiresAt.Time)
		if ttl > 0 {
			blackKey := "jwt:blacklist:" + tokenStr
			_ = l.svcCtx.Redis.Setex(blackKey, "1", int(ttl.Seconds()))
		}
	}

	return &types.LogoutResp{Success: true}, nil
}
