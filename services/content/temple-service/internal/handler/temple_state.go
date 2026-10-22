package handler

import (
	"errors"
	"github.com/askxuan/common"
	"github.com/askxuan/temple-service/internal/logic"
	"github.com/askxuan/temple-service/internal/model"
	"github.com/askxuan/temple-service/internal/svc"
	"github.com/askxuan/temple-service/internal/types"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
	"net/http"
)

func registerTempleState(server *rest.Server, sc *svc.ServiceContext) {
	server.AddRoute(rest.Route{Method: http.MethodPut, Path: "/api/v1/admin/platform/temples/:id/state", Handler: newTempleStateHandler(sc)})
}

func newTempleStateHandler(sc *svc.ServiceContext) http.HandlerFunc {
	return authorizeTempleState(sc, adminContextMiddleware(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Id string `path:"id"`
			model.TempleStatePatch
		}
		if httpx.Parse(r, &req) != nil {
			common.JsonError(w, common.ErrParamInvalid)
			return
		}
		if _, err := model.ChangeTempleState(r.Context(), sc.DB, req.Id, req.TempleStatePatch); err != nil {
			if errors.Is(err, sqlx.ErrNotFound) {
				err = common.ErrTempleNotFound
			}
			common.JsonError(w, err)
			return
		}
		detail, err := logic.PlatformTempleDetail(r.Context(), sc, &types.DetailReq{Id: req.Id})
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, detail.Temple)
	}))
}
