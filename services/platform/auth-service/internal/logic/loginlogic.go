package logic

import (
	"context"
	"github.com/askxuan/common/identity"
	"strconv"
	"time"

	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/auth-service/internal/types"
	"github.com/askxuan/common"

	"github.com/zeromicro/go-zero/core/logx"
)

// 用户状态常量（与 init.sql user 表一致）
const userStatusBanned = 0 // 0 禁用

// LoginLogic 登录逻辑
type LoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LoginLogic {
	return &LoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Login accepts a verified email/username and an Argon2id password. SMS is disabled until a provider is installed.
func (l *LoginLogic) Login(req *types.LoginReq) (*types.LoginResp, error) {
	if l.svcCtx.Accounts == nil || req.Code != "" {
		return nil, common.ErrPwdWrong
	}
	a, err := l.svcCtx.Accounts.PasswordLogin(l.ctx, "user", req.Account, req.Password)
	if err != nil {
		return nil, common.ErrPwdWrong
	}
	return l.IssueCustomer(a.UserID, a.PasswordHash)
}
func (l *LoginLogic) IssueCustomer(id int64, verifiedHash ...string) (*types.LoginResp, error) {
	u, err := l.svcCtx.UserReadonlyModel.FindByID(l.ctx, id)
	if err != nil {
		return nil, common.ErrUserNotFound
	}
	// 校验用户状态
	if u.Status == userStatusBanned {
		return nil, common.ErrUserDisabled
	}

	sid := ""
	if l.svcCtx.SessionRedis != nil {
		if len(verifiedHash) > 0 && l.svcCtx.Accounts != nil {
			sid, err = l.svcCtx.Accounts.IssueSession(l.ctx, "user", u.Id, verifiedHash[0], int(l.svcCtx.Config.Auth.RefreshExpire))
		} else {
			sid, err = identity.CreateSession(l.ctx, l.svcCtx.SessionRedis, "user", u.Id, int(l.svcCtx.Config.Auth.RefreshExpire))
		}
		if err != nil {
			return nil, common.ErrSystem
		}
	}
	// 签发 Access Token（2h）
	access, err := common.GenAccessToken(
		l.svcCtx.Config.Auth.AccessSecret,
		common.TokenInfo{
			SessionID: sid,
			UserId:    u.Id,
			Mobile:    u.Mobile,
			UserType:  "user",
			Roles:     []string{"customer"},
			ClientID:  "customer",
		},
		l.svcCtx.Config.Auth.AccessExpire,
	)
	if err != nil {
		l.Errorf("签发 access token 失败: %v", err)
		return nil, common.ErrSystem
	}

	// 签发 Refresh Token（7d）
	refresh, err := common.GenRefreshToken(
		l.svcCtx.Config.Auth.AccessSecret,
		common.TokenInfo{SessionID: sid, UserId: u.Id, UserType: "user"},
		l.svcCtx.Config.Auth.RefreshExpire,
	)
	if err != nil {
		l.Errorf("签发 refresh token 失败: %v", err)
		return nil, common.ErrSystem
	}

	// best-effort 同步用户到 OpenIM 并获取 IM token
	// 使用独立短超时 context，避免 OpenIM 响应慢拖垮整个登录请求（go-zero slow call 3s 阈值）
	var imToken string
	if l.svcCtx.IMClient != nil {
		userIDStr := "u_" + strconv.FormatInt(u.Id, 10)
		imCtx, imCancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = l.svcCtx.IMClient.RegisterUser(imCtx, userIDStr, u.Nickname, u.Avatar)
		if token, err := l.svcCtx.IMClient.GetUserToken(imCtx, userIDStr); err == nil {
			imToken = token
		}
		imCancel()
	}

	return &types.LoginResp{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    l.svcCtx.Config.Auth.AccessExpire,
		UserInfo: types.UserInfo{
			UserId:   u.Id,
			Nickname: u.Nickname,
			Avatar:   u.Avatar,
			Mobile:   u.Mobile,
		},
		IMToken: imToken,
	}, nil
}
