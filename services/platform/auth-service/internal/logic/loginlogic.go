package logic

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/auth-service/internal/types"
	"github.com/askxuan/common"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
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

// Login 登录，支持两种方式：
//  1. 手机号 + 验证码（mock 阶段验证码固定 1234）
//  2. 账号（手机号）+ 密码（MVP-1 明文比对）
func (l *LoginLogic) Login(req *types.LoginReq) (*types.LoginResp, error) {
	// 归一化账号：优先 phone，其次 account
	mobile := strings.TrimSpace(req.Phone)
	if mobile == "" {
		mobile = strings.TrimSpace(req.Account)
	}
	if mobile == "" {
		return nil, common.ErrParam
	}

	// 查询 user 表
	u, err := l.svcCtx.UserReadonlyModel.FindByMobile(l.ctx, mobile)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrUserNotFound
		}
		l.Errorf("查询用户失败 mobile=%s: %v", mobile, err)
		return nil, common.ErrSystem
	}

	// 校验凭证
	if req.Code != "" {
		// 验证码登录：mock 阶段固定 1234
		if req.Code != "1234" {
			return nil, common.ErrPwdWrong
		}
	} else {
		// 密码登录：MVP-1 明文比对
		if req.Password != u.Password {
			return nil, common.ErrPwdWrong
		}
	}

	// 校验用户状态
	if u.Status == userStatusBanned {
		return nil, common.ErrUserDisabled
	}

	// 签发 Access Token（2h）
	access, err := common.GenAccessToken(
		l.svcCtx.Config.Auth.AccessSecret,
		common.TokenInfo{
			UserId:   u.Id,
			Mobile:   u.Mobile,
			UserType: "user",
			Roles:    []string{"customer"},
			ClientID: "customer",
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
		u.Id,
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
