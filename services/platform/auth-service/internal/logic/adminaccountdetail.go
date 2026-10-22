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
	"github.com/askxuan/common/adminpolicy"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Account details expose explicit stored associations, never email-based guesses.
func AdminAccountDetail(ctx context.Context, s *svc.ServiceContext, id int64) (*types.AdminAccountDetailResp, error) {
	if id <= 0 {
		return nil, common.ErrParam
	}
	a, err := s.AdminAccountModel.FindByID(ctx, id)
	if errors.Is(err, sqlx.ErrNotFound) {
		return nil, common.ErrUserNotFound
	}
	if err != nil {
		return nil, common.ErrSystem
	}
	role, err := s.RoleModel.FindByID(ctx, a.RoleId)
	if err != nil {
		return nil, common.ErrSystem
	}
	out := &types.AdminAccountDetailResp{Account: types.AdminAccount{Id: a.Id, Account: a.Account, Name: a.Name, RoleId: a.RoleId, RoleName: role.Name, RoleCode: role.Code, AuthzScope: effectiveRoleScope(role), AuthzVersion: a.AuthzVersion, TempleId: a.TempleId, MasterId: a.MasterId, ShopId: a.ShopId, Status: a.Status, PlatformDisabled: a.PlatformDisabled, LastLoginTime: a.LastLoginTime, CreateTime: a.CreateTime}, Bindings: []types.AccountBinding{}, BlockedReasons: []string{}, Identity: types.AccountIdentity{ActivationStatus: "none", CredentialState: "unavailable"}}
	var credential struct {
		Email      string `db:"email"`
		VerifiedAt string `db:"verified_at"`
	}
	err = s.DB.QueryRowCtx(ctx, &credential, `SELECT email,COALESCE(DATE_FORMAT(verified_at,'%Y-%m-%d %H:%i:%s'),'') verified_at FROM auth_identity WHERE domain='admin' AND user_id=?`, id)
	if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
		return nil, common.ErrSystem
	}
	if err == nil {
		out.Identity.Email = credential.Email
		out.Identity.VerifiedAt = credential.VerifiedAt
		out.Identity.EmailVerified = credential.VerifiedAt != "" && !strings.HasSuffix(strings.ToLower(credential.Email), ".invalid")
		out.Identity.CanResetPassword = out.Identity.EmailVerified
	}
	if a.Password != "" {
		out.Identity.CredentialState = "legacy_password"
	}
	if out.Identity.EmailVerified {
		out.Identity.CredentialState = "verified_email"
	}
	var invite struct {
		Status string `db:"status"`
		Email  string `db:"email"`
	}
	err = s.DB.QueryRowCtx(ctx, &invite, `SELECT status,email FROM master_invitation WHERE account_id=?`, id)
	if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
		return nil, common.ErrSystem
	}
	if err == nil {
		out.Identity.ActivationStatus = invite.Status
		out.Identity.InvitationEmail = invite.Email
		if invite.Status == "pending" {
			out.Identity.CredentialState = "pending_activation"
			out.BlockedReasons = append(out.BlockedReasons, "等待本人完成邮箱验证并激活账号")
		}
		if invite.Status == "revoked" {
			out.BlockedReasons = append(out.BlockedReasons, "寺院已停用此分配账号")
		}
	}
	if a.PlatformDisabled {
		out.BlockedReasons = append(out.BlockedReasons, "平台已停用账号，请联系平台处理")
	}
	if a.Status != model.AccountStatusEnabled {
		out.BlockedReasons = append(out.BlockedReasons, "工作账号已停用")
	}
	if out.Identity.CredentialState == "unavailable" {
		out.BlockedReasons = append(out.BlockedReasons, "账号尚未配置可用登录凭据")
	}
	if role.Code == model.RoleCodePlatformService {
		out.BlockedReasons = append(out.BlockedReasons, "内部服务身份不提供人工登录")
	}
	if a.TempleId != "" {
		var t struct {
			Name     string `db:"name"`
			Status   string `db:"status"`
			Auth     string `db:"auth_status"`
			Platform string `db:"platform_status"`
			Shelf    string `db:"shelf_status"`
		}
		err = s.DB.QueryRowCtx(ctx, &t, `SELECT name,status,auth_status,platform_status,shelf_status FROM askxuan_temple.temple WHERE code=?`, a.TempleId)
		if errors.Is(err, sqlx.ErrNotFound) {
			out.Bindings = append(out.Bindings, types.AccountBinding{Type: "temple", Id: a.TempleId, Status: "missing"})
			out.BlockedReasons = append(out.BlockedReasons, "关联寺院不存在")
		} else if err != nil {
			return nil, common.ErrSystem
		} else {
			out.Bindings = append(out.Bindings, types.AccountBinding{Type: "temple", Id: a.TempleId, Name: t.Name, Status: t.Status, AuthStatus: t.Auth, PlatformStatus: t.Platform, ShelfStatus: t.Shelf})
			if (adminpolicy.IsTempleRole(role.Code) || role.Code == model.RoleCodeMaster) && (t.Auth != "certified" || t.Platform != "normal") {
				out.BlockedReasons = append(out.BlockedReasons, "关联寺院未通过审核或已被封禁")
			}
		}
	} else if adminpolicy.IsTempleRole(role.Code) {
		out.BlockedReasons = append(out.BlockedReasons, "寺院工作身份尚未关联寺院")
	}
	if a.MasterId != "" {
		var m struct {
			Name     string `db:"dharma_name"`
			Auth     string `db:"auth_status"`
			Shelf    string `db:"shelf_status"`
			Platform string `db:"platform_status"`
			Temple   string `db:"temple_code"`
		}
		err = s.DB.QueryRowCtx(ctx, &m, `SELECT dharma_name,auth_status,shelf_status,platform_status,temple_code FROM askxuan_master.master WHERE code=?`, a.MasterId)
		if errors.Is(err, sqlx.ErrNotFound) {
			out.Bindings = append(out.Bindings, types.AccountBinding{Type: "master", Id: a.MasterId, Status: "missing"})
			out.BlockedReasons = append(out.BlockedReasons, "关联师傅不存在")
		} else if err != nil {
			return nil, common.ErrSystem
		} else {
			out.Bindings = append(out.Bindings, types.AccountBinding{Type: "master", Id: a.MasterId, Name: m.Name, Status: m.Platform, AuthStatus: m.Auth, ShelfStatus: m.Shelf, PlatformStatus: m.Platform})
			if role.Code == model.RoleCodeMaster && (m.Auth != "已认证" || m.Platform != "normal") {
				out.BlockedReasons = append(out.BlockedReasons, "关联师傅未认证或已被平台封禁")
			}
			if role.Code == model.RoleCodeMaster && m.Temple != a.TempleId {
				out.BlockedReasons = append(out.BlockedReasons, "工作账号与师傅的寺院归属不一致")
			}
		}
	} else if role.Code == model.RoleCodeMaster {
		out.BlockedReasons = append(out.BlockedReasons, "师傅工作身份尚未关联师傅")
	}
	if a.ShopId > 0 {
		out.Bindings = append(out.Bindings, types.AccountBinding{Type: "shop", Id: strconv.FormatInt(a.ShopId, 10), Name: "商城 " + strconv.FormatInt(a.ShopId, 10), Status: "configured"})
	} else if role.Code == model.RoleCodeShopAdmin {
		out.BlockedReasons = append(out.BlockedReasons, "商城工作身份尚未关联商城")
	}
	out.LoginAllowed = len(out.BlockedReasons) == 0
	return out, nil
}

// Incrementing the authoritative account version invalidates access and refresh
// tokens at every live authorization check, including the operator's own token.
// Credentials, status, business bindings and orders are unchanged.
func RevokeAdminAccountSessions(ctx context.Context, s *svc.ServiceContext, id int64) (*types.AdminAccountRevokeResp, error) {
	if id <= 0 {
		return nil, common.ErrParam
	}
	result, err := s.DB.ExecCtx(ctx, `UPDATE admin_account SET authz_version=authz_version+1 WHERE id=?`, id)
	if err != nil {
		return nil, common.ErrSystem
	}
	n, err := result.RowsAffected()
	if err != nil {
		return nil, common.ErrSystem
	}
	if n != 1 {
		return nil, common.ErrUserNotFound
	}
	return &types.AdminAccountRevokeResp{Id: id, Success: true}, nil
}
