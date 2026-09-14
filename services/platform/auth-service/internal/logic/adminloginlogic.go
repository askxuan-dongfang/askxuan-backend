package logic

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/askxuan/auth-service/internal/model"
	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/auth-service/internal/types"
	"github.com/askxuan/common"
	"github.com/askxuan/common/identity"

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
// 密码只接受 Argon2id 哈希；旧演示账户需先完成凭据迁移。
func (l *AdminLoginLogic) AdminLogin(req *types.AdminLoginReq) (*types.LoginResp, error) {
	account := strings.TrimSpace(req.Account)
	if account == "" {
		return nil, common.ErrParam
	}

	var acc *model.AdminAccount
	var err error
	hash := ""
	if l.svcCtx.Accounts != nil {
		a, lookup := l.svcCtx.Accounts.Find(l.ctx, "admin", account)
		if lookup == nil {
			acc, err = l.svcCtx.AdminAccountModel.FindByID(l.ctx, a.UserID)
			hash = a.PasswordHash
		} else if !errors.Is(lookup, sqlx.ErrNotFound) {
			return nil, common.ErrSystem
		}
	}
	if acc == nil {
		acc, err = l.svcCtx.AdminAccountModel.FindByAccount(l.ctx, account)
		if acc != nil {
			hash = acc.Password
		}
	}
	if err != nil || acc == nil || !identity.CheckPassword(hash, req.Password) {
		return nil, common.ErrPwdWrong
	}
	if acc.Status != model.AccountStatusEnabled {
		return nil, common.ErrUserDisabled
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
	if role.Code == model.RoleCodeTempleAdmin {
		status, statusErr := templeStatus(l.ctx, l.svcCtx, acc.TempleId)
		if statusErr != nil {
			return nil, statusErr
		}
		if !canEnableTempleAccount(status) {
			return nil, common.ErrUserDisabled
		}
	}
	if role.Code == model.RoleCodeMaster {
		status, statusErr := masterAccountStatusFor(l.ctx, l.svcCtx, acc.MasterId)
		if statusErr != nil {
			return nil, statusErr
		}
		if !canEnableMasterAccount(status.authStatus, status.platformStatus, status.templeStatus) {
			return nil, common.ErrUserDisabled
		}
	}

	// 根据 role.Code 映射 clientId
	clientID := roleCodeToClientID(role.Code)

	// templeId 从 VARCHAR（如 "T001"）解析为 int64；字符串格式查主库 temple 表。
	var templeID int64
	if acc.TempleId != "" {
		if id, err := strconv.ParseInt(acc.TempleId, 10, 64); err == nil {
			templeID = id
		} else {
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
			// 字符串格式（如 "M001"），查主库 master 表。
			var id int64
			if err := l.svcCtx.DB.QueryRowCtx(l.ctx, &id,
				"SELECT id FROM askxuan_master.master WHERE code = ?", acc.MasterId); err == nil {
				masterID = id
			}
		}
	}

	sid := ""
	if l.svcCtx.SessionRedis != nil {
		if l.svcCtx.Accounts != nil {
			sid, err = l.svcCtx.Accounts.IssueSession(l.ctx, "admin", acc.Id, hash, int(l.svcCtx.Config.Auth.RefreshExpire))
		} else {
			sid, err = identity.CreateSession(l.ctx, l.svcCtx.SessionRedis, "admin", acc.Id, int(l.svcCtx.Config.Auth.RefreshExpire))
		}
		if err != nil {
			return nil, common.ErrSystem
		}
	}
	// 签发 Access Token（2h）
	access, err := common.GenAccessToken(
		l.svcCtx.Config.Auth.AccessSecret,
		common.TokenInfo{
			SessionID:  sid,
			UserId:     acc.Id,
			Mobile:     acc.Account,
			UserType:   "admin",
			Roles:      []string{role.Code},
			ClientID:   clientID,
			TempleID:   templeID,
			TempleCode: acc.TempleId,
			MasterID:   masterID,
		},
		l.svcCtx.Config.Auth.AccessExpire,
	)
	if err != nil {
		l.Errorf("签发 access token 失败: %v", err)
		return nil, common.ErrSystem
	}

	// Master accounts share admin_account storage, but retain their own refresh domain.
	refreshDomain := "admin"
	if role.Code == model.RoleCodeMaster {
		refreshDomain = "master"
	}
	refresh, err := common.GenRefreshToken(
		l.svcCtx.Config.Auth.AccessSecret,
		common.TokenInfo{SessionID: sid, UserId: acc.Id, UserType: refreshDomain},
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
	// 使用独立短超时 context，避免 OpenIM 响应慢拖垮整个登录请求（go-zero slow call 3s 阈值）
	var imToken string
	if l.svcCtx.IMClient != nil && masterID > 0 {
		userIDStr := "m_" + strconv.FormatInt(masterID, 10)
		masterName := acc.Name
		masterAvatar := ""
		imCtx, imCancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = l.svcCtx.IMClient.RegisterUser(imCtx, userIDStr, masterName, masterAvatar)
		if token, err := l.svcCtx.IMClient.GetUserToken(imCtx, userIDStr); err == nil {
			imToken = token
		}
		imCancel()
	}

	templeName := ""
	if acc.TempleId != "" {
		_ = l.svcCtx.DB.QueryRowCtx(l.ctx, &templeName,
			"SELECT name FROM askxuan_temple.temple WHERE code = ?", acc.TempleId)
	}

	return &types.LoginResp{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    l.svcCtx.Config.Auth.AccessExpire,
		UserInfo: types.UserInfo{
			UserId:     acc.Id,
			Nickname:   acc.Name,
			Mobile:     acc.Account,
			TempleId:   acc.TempleId,
			TempleName: templeName,
			MasterId:   acc.MasterId,
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
