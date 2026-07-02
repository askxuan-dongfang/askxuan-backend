package logic

import (
	"context"
	"strings"

	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/auth-service/internal/types"
	"github.com/askxuan/common"

	"github.com/zeromicro/go-zero/core/logx"
)

// RefreshLogic 续期逻辑
type RefreshLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRefreshLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefreshLogic {
	return &RefreshLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Refresh 校验 refresh token 并重签 access token
func (l *RefreshLogic) Refresh(req *types.RefreshReq) (*types.RefreshResp, error) {
	tokenStr := strings.TrimSpace(req.RefreshToken)
	if tokenStr == "" {
		return nil, common.ErrParam
	}

	claims, err := common.ParseToken(l.svcCtx.Config.Auth.AccessSecret, tokenStr)
	if err != nil {
		return nil, common.ErrTokenInvalid
	}
	if !claims.IsRefreshToken() {
		// access token 不能用来续期
		return nil, common.ErrTokenInvalid
	}

	// 校验 refresh token 是否在黑名单（登出过的）
	blackKey := "jwt:blacklist:" + tokenStr
	exists, _ := l.svcCtx.Redis.Get(blackKey)
	if exists == "1" {
		return nil, common.ErrTokenInvalid
	}

	// 重签 access token（refresh token 仅携带 userId，用户详细角色信息需客户端重新登录获取）
	info := common.TokenInfo{
		UserId: claims.UserId,
		Mobile: claims.Mobile,
	}
	access, err := common.GenAccessToken(
		l.svcCtx.Config.Auth.AccessSecret,
		info,
		l.svcCtx.Config.Auth.AccessExpire,
	)
	if err != nil {
		l.Errorf("重签 access token 失败: %v", err)
		return nil, common.ErrSystem
	}

	return &types.RefreshResp{
		AccessToken: access,
		ExpiresIn:   l.svcCtx.Config.Auth.AccessExpire,
	}, nil
}
