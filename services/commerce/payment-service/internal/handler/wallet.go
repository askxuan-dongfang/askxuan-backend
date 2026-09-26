package handler

import (
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/payment-service/internal/svc"
	"github.com/askxuan/payment-service/internal/wallet"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"net/http"
	"strconv"
)

func registerWallet(server *rest.Server, svcCtx *svc.ServiceContext) {
	auth := &middleware.AuthConfig{Secret: svcCtx.Config.Auth.AccessSecret}
	role := &middleware.AdminAuthConfig{AllowedRoles: []string{"customer"}}
	server.AddRoute(rest.Route{Method: http.MethodGet, Path: "/api/v1/payments/wallet", Handler: auth.AuthFunc(role.AdminAuthFunc(walletHandler(wallet.Store{DB: svcCtx.DB})))})
}
func walletHandler(store wallet.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := middleware.UserIDFromCtx(r.Context())
		if user <= 0 {
			common.JsonError(w, common.ErrUnauthorized)
			return
		}
		mode, filter := r.URL.Query().Get("mode"), r.URL.Query().Get("filter")
		if mode == "" {
			mode = "channel"
		}
		if filter == "" {
			filter = "all"
		}
		page := 1
		if raw := r.URL.Query().Get("page"); raw != "" {
			n, e := strconv.Atoi(raw)
			if e != nil {
				common.JsonError(w, common.ErrParam)
				return
			}
			page = n
		}
		if page < 1 || page > 100000 || (mode != "channel" && mode != "mock") || (filter != "all" && filter != "refunds") {
			common.JsonError(w, common.ErrParam)
			return
		}
		data, err := store.Read(r.Context(), strconv.FormatInt(user, 10), mode, filter, page)
		if err != nil {
			logx.Errorf("wallet read: %v", err)
			common.JsonError(w, common.ErrSystem)
			return
		}
		common.Ok(w, data)
	}
}
