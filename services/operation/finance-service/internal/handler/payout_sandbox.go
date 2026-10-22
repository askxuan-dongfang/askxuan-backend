package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/finance-service/internal/logic"
	"github.com/askxuan/finance-service/internal/svc"
	"github.com/askxuan/finance-service/internal/types"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
	"github.com/zeromicro/go-zero/rest/pathvar"
)

func registerPayoutSandbox(server *rest.Server, s *svc.ServiceContext) {
	auth := &middleware.AuthConfig{Secret: s.Config.AuthSecret}
	const prefix = "/api/v1/admin/finance/payout-sandbox"
	for _, r := range []struct{ method, path, action string }{{"GET", prefix, "list"}, {"GET", prefix + "/:id", "detail"}, {"POST", prefix, "create"}, {"POST", prefix + "/:id/actions", "action"}} {
		server.AddRoute(rest.Route{Method: r.method, Path: r.path, Handler: auth.AuthFunc(payoutSandboxHandler(s, r.action))})
	}
}

// Unknown fields are rejected, including real bank/account/recipient inputs.
func sandboxBody(w http.ResponseWriter, r *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return common.ErrParamInvalid
	}
	return nil
}
func payoutSandboxHandler(s *svc.ServiceContext, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := logic.RequirePayoutSandboxSuper(r.Context()); err != nil {
			common.JsonError(w, err)
			return
		}
		l := logic.NewPayoutSandboxLogic(r.Context(), s)
		var response any
		var err error
		switch action {
		case "list":
			var req types.PayoutSandboxListReq
			if err = httpx.Parse(r, &req); err == nil {
				response, err = l.List(&req)
			} else {
				err = common.ErrParamInvalid
			}
		case "detail":
			response, err = l.Detail(pathvar.Vars(r)["id"])
		case "create":
			var req types.PayoutSandboxCreateReq
			if err = sandboxBody(w, r, &req); err == nil {
				response, err = l.Create(&req)
			} else {
				err = common.ErrParamInvalid
			}
		case "action":
			var req types.PayoutSandboxActionReq
			if err = sandboxBody(w, r, &req); err == nil {
				response, err = l.Act(pathvar.Vars(r)["id"], &req)
			} else {
				err = common.ErrParamInvalid
			}
		default:
			err = common.ErrParamInvalid
		}
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, response)
	}
}
