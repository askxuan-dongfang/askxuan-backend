package logic

import (
	"context"
	"strconv"
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

	// 重签 access token：根据 UserType 重新查询用户/管理员信息，补齐 Roles/ClientID/TempleID/MasterID
	// 避免续期后角色信息丢失导致 40301 ErrForbidden
	info := common.TokenInfo{
		UserId:   claims.UserId,
		Mobile:   claims.Mobile,
		UserType: claims.UserType,
	}

	if claims.UserType == "user" {
		// C 端用户：从 user 表补齐信息
		u, err := l.svcCtx.UserReadonlyModel.FindByMobile(l.ctx, claims.Mobile)
		if err == nil && u != nil {
			info.Roles = []string{"customer"}
			info.ClientID = "customer"
		}
	} else {
		// 管理员/法师：从 admin_account 表补齐信息
		acc, err := l.svcCtx.AdminAccountModel.FindByID(l.ctx, claims.UserId)
		if err == nil && acc != nil {
			// 查询角色；失败时使用默认值 "admin"，避免续期后角色丢失
			roleCode := "admin"
			role, roleErr := l.svcCtx.RoleModel.FindByID(l.ctx, acc.RoleId)
			if roleErr == nil && role != nil && role.Code != "" {
				roleCode = role.Code
			}
			info.Roles = []string{roleCode}
			info.ClientID = roleCodeToClientID(roleCode)

			// templeId 转换：VARCHAR → int64
			if acc.TempleId != "" {
				if id, err := strconv.ParseInt(acc.TempleId, 10, 64); err == nil {
					info.TempleID = id
				} else {
					var tid int64
					if err := l.svcCtx.DB.QueryRowCtx(l.ctx, &tid,
						"SELECT id FROM askxuan_temple.temple WHERE code = ?", acc.TempleId); err == nil {
						info.TempleID = tid
					}
				}
			}

			// masterId 转换：VARCHAR → int64
			if acc.MasterId != "" {
				if id, err := strconv.ParseInt(acc.MasterId, 10, 64); err == nil {
					info.MasterID = id
				} else {
					var mid int64
					if err := l.svcCtx.DB.QueryRowCtx(l.ctx, &mid,
						"SELECT id FROM askxuan_master.master WHERE code = ?", acc.MasterId); err == nil {
						info.MasterID = mid
					}
				}
			}
		}
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
