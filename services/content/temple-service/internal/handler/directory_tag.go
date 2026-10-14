package handler

import (
	"github.com/askxuan/common"
	"github.com/askxuan/temple-service/internal/model"
	"github.com/askxuan/temple-service/internal/svc"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
	"net/http"
)

func registerDirectoryTagRoutes(server *rest.Server, ctx *svc.ServiceContext) {
	server.AddRoute(rest.Route{Method: http.MethodGet, Path: "/api/v1/beliefs/directory-tags", Handler: directoryTagList(ctx, false)})
	server.AddRoutes(rest.WithMiddleware(regionAdminOnly,
		rest.Route{Method: http.MethodGet, Path: "/api/v1/admin/platform/beliefs/directory-tags", Handler: directoryTagList(ctx, true)},
		rest.Route{Method: http.MethodPost, Path: "/api/v1/admin/platform/beliefs/directory-tags", Handler: directoryTagSave(ctx, true)},
		rest.Route{Method: http.MethodPut, Path: "/api/v1/admin/platform/beliefs/directory-tags/:code", Handler: directoryTagSave(ctx, false)},
		rest.Route{Method: http.MethodGet, Path: "/api/v1/admin/platform/beliefs/directory-tags/providers/:kind/:id", Handler: directoryTagAssign(ctx, false)},
		rest.Route{Method: http.MethodPut, Path: "/api/v1/admin/platform/beliefs/directory-tags/providers/:kind/:id", Handler: directoryTagAssign(ctx, true)},
	))
}
func directoryTagList(ctx *svc.ServiceContext, admin bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := model.NewDirectoryTagModel(ctx.DB).List(r.Context(), admin)
		if err != nil {
			common.JsonError(w, common.ErrSystem)
			return
		}
		common.Ok(w, map[string]any{"list": rows})
	}
}
func directoryTagSave(ctx *svc.ServiceContext, create bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			model.DirectoryTag
			PathCode string `path:"code,optional"`
		}
		if httpx.Parse(r, &req) != nil {
			common.JsonError(w, common.ErrParamInvalid)
			return
		}
		if !create {
			req.Code = req.PathCode
		}
		if err := model.NewDirectoryTagModel(ctx.DB).Save(r.Context(), &req.DirectoryTag, create); err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, req.DirectoryTag)
	}
}
func directoryTagAssign(ctx *svc.ServiceContext, save bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Kind  string   `path:"kind"`
			Id    string   `path:"id"`
			Codes []string `json:"codes,optional"`
		}
		if httpx.Parse(r, &req) != nil {
			common.JsonError(w, common.ErrParamInvalid)
			return
		}
		m := model.NewDirectoryTagModel(ctx.DB)
		if save {
			if err := m.Assign(r.Context(), req.Kind, req.Id, req.Codes); err != nil {
				common.JsonError(w, err)
				return
			}
		}
		codes, err := m.Assigned(r.Context(), req.Kind, req.Id)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, map[string]any{"codes": codes})
	}
}
