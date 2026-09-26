package handler

import (
	"context"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/finance-service/internal/model"
	"github.com/askxuan/finance-service/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"net/http"
	"strconv"
)

func walletOwner(ctx context.Context, kind string) (string, bool) {
	roles := middleware.RolesFromCtx(ctx)
	for _, role := range roles {
		if kind == "master" && role == "master" && middleware.MasterIDFromCtx(ctx) > 0 {
			return strconv.FormatInt(middleware.MasterIDFromCtx(ctx), 10), true
		}
		if kind == "temple" && role == "temple_admin" && middleware.TempleCodeFromCtx(ctx) != "" {
			return middleware.TempleCodeFromCtx(ctx), true
		}
	}
	return "", false
}
func registerWallet(server *rest.Server, svcCtx *svc.ServiceContext) {
	auth := &middleware.AuthConfig{Secret: svcCtx.Config.AuthSecret}
	for _, kind := range []string{"master", "temple"} {
		server.AddRoute(rest.Route{Method: http.MethodGet, Path: "/api/v1/finance/wallet/" + kind, Handler: auth.AuthFunc(providerWalletHandler(kind))})
	}
}
func providerWalletHandler(kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		target, ok := walletOwner(r.Context(), kind)
		if !ok {
			common.JsonError(w, common.ErrForbidden)
			return
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
		if page < 1 || page > 100000 {
			common.JsonError(w, common.ErrParam)
			return
		}
		data, err := model.ReadProviderWallet(r.Context(), kind, target, page)
		if err != nil {
			logx.Errorf("provider wallet: %v", err)
			common.JsonError(w, common.ErrSystem)
			return
		}
		common.Ok(w, data)
	}
}
