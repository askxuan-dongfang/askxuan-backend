package handler

import (
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/finance-service/internal/model"
	"github.com/askxuan/finance-service/internal/svc"
	"github.com/zeromicro/go-zero/rest"
	"net/http"
)

func registerTempleReport(server *rest.Server, s *svc.ServiceContext) {
	auth := &middleware.AuthConfig{Secret: s.Config.AuthSecret}
	server.AddRoute(rest.Route{Method: http.MethodGet, Path: "/api/v1/finance/wallet/temple/report", Handler: auth.AuthFunc(func(w http.ResponseWriter, r *http.Request) {
		code, ok := walletOwner(r.Context(), "temple")
		if !ok {
			common.JsonError(w, common.ErrForbidden)
			return
		}
		ctx := r.Context()
		mode := r.URL.Query().Get("mode")
		if mode != "" && mode != "accounting_only" && mode != "demo" {
			common.JsonError(w, common.ErrParam)
			return
		}
		if mode == "demo" {
			if !s.Config.DemoWalletEnabled {
				common.JsonError(w, common.ErrForbidden)
				return
			}
			ctx = model.DemoLedgerContext(ctx)
		}
		out, err := model.ReadTempleSettlementReport(ctx, code, r.URL.Query().Get("startTime"), r.URL.Query().Get("endTime"))
		if err != nil {
			if _, ok := err.(*common.BizError); !ok {
				err = common.ErrSystem
			}
			common.JsonError(w, err)
			return
		}
		common.Ok(w, out)
	})})
}
