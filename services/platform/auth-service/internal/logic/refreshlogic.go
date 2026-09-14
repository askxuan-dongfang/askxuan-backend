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
	"github.com/askxuan/common/identity"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// RefreshLogic renews only the identity domain bound at sign-in.
type RefreshLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRefreshLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RefreshLogic {
	return &RefreshLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *RefreshLogic) Refresh(req *types.RefreshReq) (*types.RefreshResp, error) {
	tokenStr := strings.TrimSpace(req.RefreshToken)
	if tokenStr == "" {
		return nil, common.ErrParam
	}
	claims, err := common.ParseToken(l.svcCtx.Config.Auth.AccessSecret, tokenStr)
	if err != nil || !validRefreshIdentity(claims) {
		return nil, common.ErrTokenInvalid
	}

	if l.svcCtx.SessionRedis != nil {
		if e := identity.CheckSession(l.ctx, l.svcCtx.SessionRedis, claims.SessionID, identity.SessionDomain(claims.UserType, claims.Roles), claims.UserId); e != nil {
			return nil, common.ErrTokenInvalid
		}
	}
	// A failed revocation lookup is a temporary service error, never permission to renew.
	if l.svcCtx.Redis == nil {
		return nil, common.ErrSystem
	}
	blocked, err := l.svcCtx.Redis.GetCtx(l.ctx, "jwt:blacklist:"+tokenStr)
	if err != nil {
		return nil, common.ErrSystem
	}
	if blocked == "1" {
		return nil, common.ErrTokenInvalid
	}

	info, err := l.refreshIdentity(claims)
	if err != nil {
		return nil, err
	}
	info.SessionID = claims.SessionID
	access, err := common.GenAccessToken(l.svcCtx.Config.Auth.AccessSecret, info, l.svcCtx.Config.Auth.AccessExpire)
	if err != nil {
		return nil, common.ErrSystem
	}
	return &types.RefreshResp{AccessToken: access, ExpiresIn: l.svcCtx.Config.Auth.AccessExpire}, nil
}

func validRefreshIdentity(claims *common.CustomClaims) bool {
	if claims == nil || !claims.IsRefreshToken() || claims.UserId <= 0 || claims.ExpiresAt == nil {
		return false
	}
	switch claims.UserType {
	case "user", "admin", "master":
		return true
	default:
		return false // Legacy tokens without a domain must sign in again.
	}
}

func (l *RefreshLogic) refreshIdentity(claims *common.CustomClaims) (common.TokenInfo, error) {
	if claims.UserType == "user" {
		u, err := l.svcCtx.UserReadonlyModel.FindByID(l.ctx, claims.UserId)
		if err != nil {
			return common.TokenInfo{}, refreshLookupError(err)
		}
		if u == nil || u.Id != claims.UserId || u.Status == userStatusBanned {
			return common.TokenInfo{}, common.ErrTokenInvalid
		}
		return common.TokenInfo{UserId: u.Id, Mobile: u.Mobile, UserType: "user", Roles: []string{model.RoleCodeCustomer}, ClientID: "customer"}, nil
	}
	// Never infer this branch from an absent or unrecognized identity domain.
	if claims.UserType != "admin" && claims.UserType != "master" {
		return common.TokenInfo{}, common.ErrTokenInvalid
	}
	acc, err := l.svcCtx.AdminAccountModel.FindByID(l.ctx, claims.UserId)
	if err != nil {
		return common.TokenInfo{}, refreshLookupError(err)
	}
	if acc == nil || acc.Id != claims.UserId || acc.Status != model.AccountStatusEnabled {
		return common.TokenInfo{}, common.ErrTokenInvalid
	}
	role, err := l.svcCtx.RoleModel.FindByID(l.ctx, acc.RoleId)
	if err != nil {
		return common.TokenInfo{}, refreshLookupError(err)
	}
	if role == nil || !refreshRoleMatchesDomain(claims.UserType, role.Code) {
		return common.TokenInfo{}, common.ErrTokenInvalid
	}

	// Refresh checks the same bound-entity eligibility as a fresh administrator login.
	if role.Code == model.RoleCodeTempleAdmin {
		if acc.TempleId == "" {
			return common.TokenInfo{}, common.ErrTokenInvalid
		}
		status, err := templeStatus(l.ctx, l.svcCtx, acc.TempleId)
		if err != nil {
			return common.TokenInfo{}, refreshEligibilityError(err)
		}
		if !canEnableTempleAccount(status) {
			return common.TokenInfo{}, common.ErrTokenInvalid
		}
	}
	if role.Code == model.RoleCodeMaster {
		if acc.MasterId == "" {
			return common.TokenInfo{}, common.ErrTokenInvalid
		}
		status, err := masterAccountStatusFor(l.ctx, l.svcCtx, acc.MasterId)
		if err != nil {
			return common.TokenInfo{}, refreshEligibilityError(err)
		}
		if !canEnableMasterAccount(status.authStatus, status.platformStatus, status.templeStatus) {
			return common.TokenInfo{}, common.ErrTokenInvalid
		}
	}

	info := common.TokenInfo{UserId: acc.Id, Mobile: acc.Account, UserType: "admin", Roles: []string{role.Code}, ClientID: roleCodeToClientID(role.Code), TempleCode: acc.TempleId}
	// Access tokens retain the existing admin+master representation for native/H5 clients.
	if acc.TempleId != "" {
		info.TempleID, err = refreshBoundID(l.ctx, l.svcCtx.DB, acc.TempleId, "SELECT id FROM askxuan_temple.temple WHERE code = ?")
		if err != nil {
			return common.TokenInfo{}, err
		}
	}
	if acc.MasterId != "" {
		info.MasterID, err = refreshBoundID(l.ctx, l.svcCtx.DB, acc.MasterId, "SELECT id FROM askxuan_master.master WHERE code = ?")
		if err != nil {
			return common.TokenInfo{}, err
		}
	}
	return info, nil
}

func refreshRoleMatchesDomain(domain, role string) bool {
	if domain == "master" {
		return role == model.RoleCodeMaster
	}
	if domain != "admin" {
		return false
	}
	switch role {
	case model.RoleCodePlatformSuper, model.RoleCodePlatformService, model.RoleCodeTempleAdmin, model.RoleCodeShopAdmin:
		return true
	default:
		return false
	}
}

func refreshBoundID(ctx context.Context, db sqlx.SqlConn, code, query string) (int64, error) {
	id, err := strconv.ParseInt(code, 10, 64)
	if err != nil {
		if err := db.QueryRowCtx(ctx, &id, query, code); err != nil {
			return 0, refreshLookupError(err)
		}
	}
	if id <= 0 {
		return 0, common.ErrTokenInvalid
	}
	return id, nil
}

func refreshLookupError(err error) error {
	if errors.Is(err, sqlx.ErrNotFound) {
		return common.ErrTokenInvalid
	}
	return common.ErrSystem
}

func refreshEligibilityError(err error) error {
	if errors.Is(err, common.ErrSystem) {
		return common.ErrSystem
	}
	return common.ErrTokenInvalid
}
