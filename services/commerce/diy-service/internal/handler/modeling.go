package handler

import (
	"encoding/json"
	"net/http"

	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/diy-service/internal/modeling"
	"github.com/askxuan/diy-service/internal/svc"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func modelingSuper(r *http.Request) bool {
	c := middleware.AdminClaimsFromCtx(r.Context())
	return c != nil && c.UserType == "admin" && c.UserId > 0 && c.HasRole("platform_super")
}
func registerModeling(server *rest.Server, s *svc.ServiceContext) {
	auth := &middleware.AuthConfig{Secret: s.Config.AuthSecret}
	admin := &middleware.AdminAuthConfig{AllowedRoles: []string{"shop_admin", "platform_super"}}
	wrap := func(next http.HandlerFunc) http.HandlerFunc {
		return auth.AuthFunc(admin.AdminAuthFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := middleware.AdminClaimsFromCtx(r.Context())
			if claims == nil || claims.UserType != "admin" || claims.UserId <= 0 {
				common.JsonError(w, common.ErrRoleForbidden)
				return
			}
			if s.Modeling == nil {
				common.JsonError(w, common.ErrDependencyUnavailable)
				return
			}
			next(w, r)
		}))
	}
	server.AddRoutes([]rest.Route{
		{Method: http.MethodGet, Path: "/api/v1/admin/diy/modeling/config", Handler: wrap(func(w http.ResponseWriter, r *http.Request) {
			out, e := s.Modeling.PublicConfig(r.Context(), modelingSuper(r))
			if e != nil {
				common.JsonError(w, common.ErrSystem)
				return
			}
			common.Ok(w, out)
		})},
		{Method: http.MethodPut, Path: "/api/v1/admin/diy/modeling/config", Handler: wrap(func(w http.ResponseWriter, r *http.Request) {
			if !modelingSuper(r) {
				common.JsonError(w, common.ErrRoleForbidden)
				return
			}
			var u modeling.ConfigUpdate
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&u) != nil {
				common.JsonError(w, common.ErrParam)
				return
			}
			out, e := s.Modeling.UpdateConfig(r.Context(), middleware.UserIDFromCtx(r.Context()), u)
			if e != nil {
				common.JsonError(w, e)
				return
			}
			common.Ok(w, out)
		})},
		{Method: http.MethodPost, Path: "/api/v1/admin/diy/modeling/tasks", Handler: wrap(func(w http.ResponseWriter, r *http.Request) {
			var u modeling.CreateTask
			if json.NewDecoder(http.MaxBytesReader(w, r.Body, 12288)).Decode(&u) != nil {
				common.JsonError(w, common.ErrParam)
				return
			}
			out, e := s.Modeling.Create(r.Context(), middleware.UserIDFromCtx(r.Context()), u)
			if e != nil {
				common.JsonError(w, e)
				return
			}
			common.Ok(w, out)
		})},
		{Method: http.MethodGet, Path: "/api/v1/admin/diy/modeling/tasks/:id", Handler: wrap(func(w http.ResponseWriter, r *http.Request) {
			var p struct {
				ID string `path:"id"`
			}
			if httpx.ParsePath(r, &p) != nil {
				common.JsonError(w, common.ErrParam)
				return
			}
			out, e := s.Modeling.Task(r.Context(), p.ID, middleware.UserIDFromCtx(r.Context()), modelingSuper(r))
			if e != nil {
				common.JsonError(w, e)
				return
			}
			common.Ok(w, out)
		})},
	})
}
