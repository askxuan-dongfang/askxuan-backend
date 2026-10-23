package logic

import (
	"context"
	"sort"

	"github.com/askxuan/auth-service/internal/model"
	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/askxuan/common/adminpolicy"
)

// Built-in role scope cannot be changed through database metadata.
func effectiveRoleScope(r *model.Role) string {
	if adminpolicy.Builtin(r.Code) {
		return adminpolicy.Scope(r.Code)
	}
	return r.AuthzScope
}
func authorizationSnapshot(ctx context.Context, s *svc.ServiceContext, a *model.AdminAccount, r *model.Role) (common.TokenInfo, error) {
	info := common.TokenInfo{RoleID: r.Id, AccountVersion: a.AuthzVersion, RoleVersion: r.AuthzVersion, AuthzScope: effectiveRoleScope(r), Permissions: []string{}}
	if adminpolicy.IsRetiredTempleRole(r.Code) {
		return common.TokenInfo{}, common.ErrRoleForbidden
	}
	if adminpolicy.Builtin(r.Code) {
		return info, nil
	}
	if info.AuthzScope != "platform" || s.DB == nil {
		return common.TokenInfo{}, common.ErrRoleForbidden
	}
	var rows []struct {
		Code string `db:"code"`
	}
	if err := s.DB.QueryRowsCtx(ctx, &rows, `SELECT p.code FROM role_permission rp JOIN permission p ON p.id=rp.permission_id WHERE rp.role_id=? ORDER BY p.code`, r.Id); err != nil {
		return common.TokenInfo{}, common.ErrSystem
	}
	for _, p := range rows {
		if adminpolicy.Assignable(p.Code) {
			info.Permissions = append(info.Permissions, p.Code)
		}
	}
	sort.Strings(info.Permissions)
	return info, nil
}
func authorizationVersionMatches(c *common.CustomClaims, a *model.AdminAccount, r *model.Role) bool {
	if adminpolicy.IsRetiredTempleRole(r.Code) || r.Id != a.RoleId {
		return false
	}
	if c.AccountVersion != a.AuthzVersion || c.RoleVersion != r.AuthzVersion {
		return false
	}
	// Pre-migration built-in sessions remain usable only at version zero. A new
	// custom role always needs its signed role ID and scope, even if empty.
	if len(c.Roles) > 0 && (len(c.Roles) != 1 || c.Roles[0] != r.Code) {
		return false
	}
	if c.RoleID == 0 {
		return adminpolicy.Builtin(r.Code) && a.AuthzVersion == 0 && r.AuthzVersion == 0
	}
	if c.RoleID != a.RoleId || c.AuthzScope != effectiveRoleScope(r) {
		return false
	}
	if len(c.Roles) > 0 && (len(c.Roles) != 1 || c.Roles[0] != r.Code) {
		return false
	}
	return true
}

// CurrentAdminAccess validates the same live account/binding/version snapshot as
// refresh, without issuing a token or calling the gateway (no recursive check).
func CurrentAdminAccess(ctx context.Context, s *svc.ServiceContext, c *common.CustomClaims) (common.TokenInfo, error) {
	if c == nil || c.UserType != "admin" || c.IsRefreshToken() || c.UserId <= 0 {
		return common.TokenInfo{}, common.ErrTokenInvalid
	}
	if c.HasRetiredTempleRole() {
		return common.TokenInfo{}, common.ErrSessionRevoked
	}
	copyClaims := *c
	if c.HasRole(model.RoleCodeMaster) {
		copyClaims.UserType = "master"
	}
	return NewRefreshLogic(ctx, s).refreshIdentity(&copyClaims)
}
