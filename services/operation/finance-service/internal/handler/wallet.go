package handler

import (
	"context"
	"encoding/json"
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
	if svcCtx.Config.DemoWalletEnabled {
		admin := &middleware.AdminAuthConfig{AllowedRoles: []string{"platform_super"}}
		server.AddRoute(rest.Route{Method: http.MethodGet, Path: "/api/v1/finance/wallet/platform/demo", Handler: auth.AuthFunc(admin.AdminAuthFunc(func(w http.ResponseWriter, r *http.Request) {
			data, err := model.ReadDemoPlatformWallet(r.Context())
			if err != nil {
				logx.Error("demo finance summary failed")
				common.JsonError(w, common.ErrSystem)
				return
			}
			common.Ok(w, data)
		}))})
	}
	for _, kind := range []string{"master", "temple"} {
		server.AddRoute(rest.Route{Method: http.MethodGet, Path: "/api/v1/finance/wallet/" + kind, Handler: auth.AuthFunc(providerWalletHandler(kind, svcCtx.Config.DemoWalletEnabled))})
		if svcCtx.Config.DemoWalletEnabled {
			server.AddRoute(rest.Route{Method: http.MethodPost, Path: "/api/v1/finance/wallet/" + kind + "/simulate-payout", Handler: auth.AuthFunc(func(w http.ResponseWriter, r *http.Request) {
				target, ok := walletOwner(r.Context(), kind)
				if !ok {
					common.JsonError(w, common.ErrForbidden)
					return
				}
				var req struct {
					SettlementID int64 `json:"settlementId"`
				}
				if json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req) != nil {
					common.JsonError(w, common.ErrParam)
					return
				}
				if err := model.SimulatePayout(r.Context(), kind, target, req.SettlementID); err != nil {
					if b, ok := err.(*common.BizError); ok {
						common.JsonError(w, b)
					} else {
						logx.Error("demo payout failed")
						common.JsonError(w, common.ErrSystem)
					}
					return
				}
				common.Ok(w, map[string]any{"mode": "demo", "status": "success"})
			})})
		}
	}
}
func providerWalletHandler(kind string, demoEnabled ...bool) http.HandlerFunc {
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
		ctx := r.Context()
		if r.URL.Query().Get("mode") == "demo" {
			if len(demoEnabled) == 0 || !demoEnabled[0] {
				common.JsonError(w, common.ErrForbidden)
				return
			}
			ctx = model.DemoLedgerContext(ctx)
		}
		data, err := model.ReadProviderWallet(ctx, kind, target, page)
		if err != nil {
			logx.Errorf("provider wallet: %v", err)
			common.JsonError(w, common.ErrSystem)
			return
		}
		common.Ok(w, data)
	}
}
