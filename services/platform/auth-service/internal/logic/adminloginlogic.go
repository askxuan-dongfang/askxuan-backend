package logic

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/askxuan/auth-service/internal/model"
	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/auth-service/internal/types"
	"github.com/askxuan/common"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// AdminLoginLogic 管理台登录逻辑
type AdminLoginLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminLoginLogic {
	return &AdminLoginLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AdminLogin 管理台登录（account + password）
// MVP-1 阶段明文比对密码
func (l *AdminLoginLogic) AdminLogin(req *types.AdminLoginReq) (*types.LoginResp, error) {
	account := strings.TrimSpace(req.Account)
	if account == "" {
		return nil, common.ErrParam
	}

	// 查询 admin_account 表
	acc, err := l.svcCtx.AdminAccountModel.FindByAccount(l.ctx, account)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrUserNotFound
		}
		l.Errorf("查询管理台账号失败 account=%s: %v", account, err)
		return nil, common.ErrSystem
	}

	// 校验账号状态
	if acc.Status != model.AccountStatusEnabled {
		return nil, common.ErrUserDisabled
	}

	// MVP-1 明文比对密码
	if req.Password != acc.Password {
		return nil, common.ErrPwdWrong
	}

	// 查询角色，获取 code
	role, err := l.svcCtx.RoleModel.FindByID(l.ctx, acc.RoleId)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			l.Errorf("管理台账号 role_id 未关联到角色 account=%s roleId=%d", account, acc.RoleId)
			return nil, common.ErrForbidden
		}
		l.Errorf("查询角色失败 roleId=%d: %v", acc.RoleId, err)
		return nil, common.ErrSystem
	}

	// 根据 role.Code 映射 clientId
	clientID := roleCodeToClientID(role.Code)

	// templeId 从 VARCHAR（如 "T001"）解析为 int64；字符串格式需跨库查 askxuan_temple.temple
	var templeID int64
	if acc.TempleId != "" {
		if id, err := strconv.ParseInt(acc.TempleId, 10, 64); err == nil {
			templeID = id
		} else {
			// 字符串格式（如 "T001"），查 temple 表（跨库 askxuan_temple.temple）
			var tid int64
			if err := l.svcCtx.DB.QueryRowCtx(l.ctx, &tid,
				"SELECT id FROM askxuan_temple.temple WHERE code = ?", acc.TempleId); err == nil {
				templeID = tid
			}
		}
	}

	// masterId：admin_account.master_id 是 VARCHAR（如 "M001"），需查 master 表获取 int64 id
	var masterID int64
	if acc.MasterId != "" {
		// 先尝试直接解析为数字
		if id, err := strconv.ParseInt(acc.MasterId, 10, 64); err == nil {
			masterID = id
		} else {
			// 字符串格式（如 "M001"），查 master 表（跨库 askxuan_master.master）
		var id int64
		if err := l.svcCtx.DB.QueryRowCtx(l.ctx, &id,
			"SELECT id FROM askxuan_master.master WHERE code = ?", acc.MasterId); err == nil {
			masterID = id
		}
		}
	}

	// 签发 Access Token（2h）
	access, err := common.GenAccessToken(
		l.svcCtx.Config.Auth.AccessSecret,
		common.TokenInfo{
			UserId:   acc.Id,
			Mobile:   acc.Account,
			UserType: "admin",
			Roles:    []string{role.Code},
			ClientID: clientID,
			TempleID: templeID,
			MasterID: masterID,
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
		acc.Id,
		l.svcCtx.Config.Auth.RefreshExpire,
	)
	if err != nil {
		l.Errorf("签发 refresh token 失败: %v", err)
		return nil, common.ErrSystem
	}

	// 更新最后登录时间（失败不影响登录）
	if err := l.svcCtx.AdminAccountModel.UpdateLastLogin(l.ctx, acc.Id); err != nil {
		l.Errorf("更新最后登录时间失败 id=%d: %v", acc.Id, err)
	}

	// best-effort 同步法师到 OpenIM 并获取 IM token（仅法师角色需要 imToken）
	var imToken string
	if l.svcCtx.IMClient != nil && masterID > 0 {
		userIDStr := "m_" + strconv.FormatInt(masterID, 10)
		masterName := acc.Name
		masterAvatar := ""
		_ = l.svcCtx.IMClient.RegisterUser(l.ctx, userIDStr, masterName, masterAvatar)
		if token, err := l.svcCtx.IMClient.GetUserToken(l.ctx, userIDStr); err == nil {
			imToken = token
		}
	}

	return &types.LoginResp{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    l.svcCtx.Config.Auth.AccessExpire,
		UserInfo: types.UserInfo{
			UserId:   acc.Id,
			Nickname: acc.Name,
			Mobile:   acc.Account,
		},
		IMToken: imToken,
	}, nil
}

// roleCodeToClientID 根据角色编码映射端标识
func roleCodeToClientID(roleCode string) string {
	switch roleCode {
	case model.RoleCodePlatformSuper, model.RoleCodePlatformService:
		return "platform-admin"
	case model.RoleCodeTempleAdmin:
		return "temple-admin"
	case model.RoleCodeMaster:
		return "master"
	case model.RoleCodeShopAdmin:
		return "shop-admin"
	default:
		return "platform-admin"
	}
}
