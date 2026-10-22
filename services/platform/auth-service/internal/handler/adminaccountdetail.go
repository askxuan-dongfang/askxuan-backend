package handler

import (
	"github.com/askxuan/auth-service/internal/logic"
	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/auth-service/internal/types"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
	"net/http"
)

func registerAccountDetails(server *rest.Server, s *svc.ServiceContext) {
	server.AddRoute(rest.Route{Method: http.MethodGet, Path: "/api/v1/admin/auth/accounts/:id", Handler: func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminAccountDetailReq
		if httpx.ParsePath(r, &req) != nil || req.Id <= 0 {
			common.JsonError(w, common.ErrParam)
			return
		}
		out, err := logic.AdminAccountDetail(r.Context(), s, req.Id)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, out)
	}})
	server.AddRoute(rest.Route{Method: http.MethodPost, Path: "/api/v1/admin/auth/accounts/:id/revoke-sessions", Handler: func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminAccountDetailReq
		if httpx.ParsePath(r, &req) != nil || req.Id <= 0 {
			common.JsonError(w, common.ErrParam)
			return
		}
		out, err := logic.RevokeAdminAccountSessions(r.Context(), s, req.Id)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, out)
	}})
}
