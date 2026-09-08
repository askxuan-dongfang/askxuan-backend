package handler

import (
	"encoding/json"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/order-service/internal/model"
	"github.com/askxuan/order-service/internal/svc"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
	"net/http"
	"strconv"
)

func registerCommerce(server *rest.Server, s *svc.ServiceContext) {
	auth := &middleware.AuthConfig{Secret: s.Config.AuthSecret}
	customer := &middleware.AdminAuthConfig{AllowedRoles: []string{"customer"}}
	admin := &middleware.AdminAuthConfig{AllowedRoles: []string{"shop_admin", "platform_super"}}
	for _, op := range []struct {
		method, path, action string
		admin                bool
	}{
		{"GET", "/api/v1/orders/:id/returns", "list", false},
		{"PUT", "/api/v1/orders/returns/:id/ship", "ship", false},
		{"PUT", "/api/v1/admin/orders/returns/:id/receive", "receive", true},
	} {
		h := commerceReturnHandler(s, op.action)
		if op.admin {
			h = admin.AdminAuthFunc(h)
		} else {
			h = customer.AdminAuthFunc(h)
		}
		server.AddRoute(rest.Route{Method: op.method, Path: op.path, Handler: auth.AuthFunc(h)})
	}
}
func commerceReturnHandler(s *svc.ServiceContext, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID int64 `path:"id"`
		}
		if httpx.ParsePath(r, &req) != nil || req.ID < 1 {
			common.JsonError(w, common.ErrParam)
			return
		}
		user := strconv.FormatInt(middleware.UserIDFromCtx(r.Context()), 10)
		if action == "list" {
			o, err := s.ShopOrderModel.FindOne(r.Context(), req.ID)
			if err != nil {
				common.JsonError(w, common.ErrOrderNotFound)
				return
			}
			if o.UserId != user {
				common.JsonError(w, common.ErrForbidden)
				return
			}
			rows, err := model.ReturnDetails(r.Context(), s.DB, req.ID)
			if err != nil {
				common.JsonError(w, common.ErrSystem)
				return
			}
			common.Ok(w, rows)
			return
		}
		var body struct {
			Carrier  string `json:"carrier"`
			Tracking string `json:"trackingNo"`
		}
		if action == "ship" && json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&body) != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		if action == "receive" {
			user = ""
		}
		if err := model.MoveReturn(r.Context(), s.DB, req.ID, user, action, body.Carrier, body.Tracking, ""); err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, map[string]bool{"success": true})
	}
}
