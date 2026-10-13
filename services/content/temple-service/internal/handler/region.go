package handler

import (
	"net/http"
	"strings"

	"github.com/askxuan/common"
	"github.com/askxuan/temple-service/internal/model"
	"github.com/askxuan/temple-service/internal/svc"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func registerRegionRoutes(server *rest.Server, svcCtx *svc.ServiceContext) {
	server.AddRoute(rest.Route{Method: http.MethodGet, Path: "/api/v1/regions", Handler: regionList(svcCtx, false)})
	server.AddRoutes(rest.WithMiddleware(regionAdminOnly,
		rest.Route{Method: http.MethodGet, Path: "/api/v1/admin/platform/regions", Handler: regionList(svcCtx, true)},
		rest.Route{Method: http.MethodPost, Path: "/api/v1/admin/platform/regions", Handler: regionSave(svcCtx, true)},
		rest.Route{Method: http.MethodPut, Path: "/api/v1/admin/platform/regions/:code", Handler: regionSave(svcCtx, false)},
		rest.Route{Method: http.MethodPut, Path: "/api/v1/admin/platform/regions/providers/:kind/:id", Handler: regionAssign(svcCtx)},
	))
}

func regionAdminOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Gateway authenticates the admin and replaces client-supplied identity headers.
		for _, role := range strings.Split(r.Header.Get("X-User-Roles"), ",") {
			if strings.TrimSpace(role) == "platform_super" && svc.ParseHeaderToInt64(r.Header.Get("X-User-Id")) > 0 {
				next(w, r)
				return
			}
		}
		common.JsonError(w, common.ErrForbidden)
	}
}

func regionList(svcCtx *svc.ServiceContext, admin bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := model.NewRegionModel(svcCtx.DB).List(r.Context(), admin)
		if err != nil {
			common.JsonError(w, common.ErrSystem)
			return
		}
		common.Ok(w, map[string]any{"list": list})
	}
}

func regionSave(svcCtx *svc.ServiceContext, create bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Code     string `json:"code,optional"`
			PathCode string `path:"code,optional"`
			Name     string `json:"name"`
			Sort     int    `json:"sort"`
			Status   string `json:"status"`
		}
		if httpx.Parse(r, &req) != nil {
			common.JsonError(w, common.ErrParamInvalid)
			return
		}
		if !create {
			req.Code = req.PathCode
		}
		p := model.RegionTag{Code: req.Code, Name: req.Name, Sort: req.Sort, Status: req.Status}
		if err := model.NewRegionModel(svcCtx.DB).Save(r.Context(), &p, create); err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, p)
	}
}

func regionAssign(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Kind       string `path:"kind"`
			Id         string `path:"id"`
			RegionCode string `json:"regionCode"`
		}
		if httpx.Parse(r, &req) != nil {
			common.JsonError(w, common.ErrParamInvalid)
			return
		}
		if err := model.NewRegionModel(svcCtx.DB).Assign(r.Context(), req.Kind, req.Id, req.RegionCode); err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, map[string]string{"regionCode": req.RegionCode})
	}
}
