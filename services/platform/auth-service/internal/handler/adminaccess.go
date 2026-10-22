package handler

import (
	"net/http"
	"strings"

	"github.com/askxuan/auth-service/internal/logic"
	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/askxuan/common/identity"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func currentAdminClaims(s *svc.ServiceContext, r *http.Request) (*common.CustomClaims, common.TokenInfo, error) {
	raw := r.Header.Get("Authorization")
	if !strings.HasPrefix(raw, "Bearer ") {
		return nil, common.TokenInfo{}, common.ErrUnauthorized
	}
	claims, err := common.ParseToken(s.Config.Auth.AccessSecret, strings.TrimPrefix(raw, "Bearer "))
	if err != nil || claims.IsRefreshToken() || claims.UserType != "admin" {
		return nil, common.TokenInfo{}, common.ErrTokenInvalid
	}
	// Production access introspection must never fall back to signature-only mode.
	if s.SessionRedis == nil {
		return nil, common.TokenInfo{}, common.ErrSystem
	}
	if err = identity.CheckSession(r.Context(), s.SessionRedis, claims.SessionID, identity.SessionDomain(claims.UserType, claims.Roles), claims.UserId); err != nil {
		return nil, common.TokenInfo{}, common.ErrTokenInvalid
	}
	info, err := logic.CurrentAdminAccess(r.Context(), s, claims)
	return claims, info, err
}
func registerAdminAccess(server *rest.Server, s *svc.ServiceContext) {
	registerTempleTeam(server, s)
	// Keep privileged account/role endpoints safe even on a direct service connection.
	server.Use(func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/v1/admin/auth/") {
				claims, _, err := currentAdminClaims(s, r)
				if err != nil {
					common.JsonError(w, err)
					return
				}
				if !claims.HasRole("platform_super") {
					common.JsonError(w, common.ErrRoleForbidden)
					return
				}
			}
			next(w, r)
		}
	})
	server.AddRoute(rest.Route{Method: http.MethodGet, Path: "/api/v1/auth/access", Handler: func(w http.ResponseWriter, r *http.Request) {
		_, info, err := currentAdminClaims(s, r)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		templeName := ""
		if info.TempleCode != "" {
			if err := s.DB.QueryRowCtx(r.Context(), &templeName, "SELECT name FROM askxuan_temple.temple WHERE code=?", info.TempleCode); err != nil {
				common.JsonError(w, common.ErrSystem)
				return
			}
		}
		common.Ok(w, map[string]any{"userId": info.UserId, "roles": info.Roles, "clientId": info.ClientID, "roleId": info.RoleID, "accountVersion": info.AccountVersion, "roleVersion": info.RoleVersion, "authzScope": info.AuthzScope, "permissions": info.Permissions, "templeCode": info.TempleCode, "templeName": templeName})
	}})
	server.AddRoute(rest.Route{Method: http.MethodGet, Path: "/api/v1/admin/auth/roles/:id/permissions", Handler: func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID int64 `path:"id"`
		}
		if httpx.ParsePath(r, &req) != nil || req.ID <= 0 {
			common.JsonError(w, common.ErrParam)
			return
		}
		out, err := logic.GetRolePermissions(r.Context(), s, req.ID)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, out)
	}})
	server.AddRoute(rest.Route{Method: http.MethodPut, Path: "/api/v1/admin/auth/roles/:id/permissions", Handler: func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID              int64    `path:"id"`
			Revision        int64    `json:"revision"`
			PermissionCodes []string `json:"permissionCodes"`
		}
		if httpx.Parse(r, &req) != nil || req.ID <= 0 || req.PermissionCodes == nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		out, err := logic.SetRolePermissions(r.Context(), s, req.ID, req.Revision, req.PermissionCodes)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, out)
	}})
}
