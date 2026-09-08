package handler

import (
	"encoding/json"
	"errors"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/payment-service/internal/points"
	"github.com/askxuan/payment-service/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
	"net/http"
	"strconv"
)

func registerPoints(server *rest.Server, svcCtx *svc.ServiceContext) {
	auth := &middleware.AuthConfig{Secret: svcCtx.Config.Auth.AccessSecret}
	customer := &middleware.AdminAuthConfig{AllowedRoles: []string{"customer"}}
	admin := &middleware.AdminAuthConfig{AllowedRoles: []string{"shop_admin", "platform_super"}}
	store := points.Store{DB: svcCtx.DB}
	for _, isAdmin := range []bool{false, true} {
		prefix := "/api/v1/points"
		if isAdmin {
			prefix = "/api/v1/admin/points"
		}
		operations := []struct{ method, path, action string }{
			{"GET", "/products", "products"}, {"GET", "/orders", "orders"},
			{"POST", "/orders/:id/cancel", "cancel"},
		}
		if isAdmin {
			operations = append(operations, struct{ method, path, action string }{"POST", "/products", "save"}, struct{ method, path, action string }{"PUT", "/products/:id", "save"}, struct{ method, path, action string }{"POST", "/orders/:id/ship", "ship"}, struct{ method, path, action string }{"GET", "/report", "report"})
		} else {
			operations = append(operations, struct{ method, path, action string }{"GET", "", "account"}, struct{ method, path, action string }{"GET", "/ledger", "ledger"}, struct{ method, path, action string }{"POST", "/orders", "redeem"}, struct{ method, path, action string }{"POST", "/orders/:id/complete", "complete"})
		}
		for _, op := range operations {
			handler := pointsHandler(store, isAdmin, op.action)
			if isAdmin {
				handler = admin.AdminAuthFunc(handler)
			} else {
				handler = customer.AdminAuthFunc(handler)
			}
			server.AddRoute(rest.Route{Method: op.method, Path: prefix + op.path, Handler: auth.AuthFunc(handler)})
		}
	}
}
func pointsHandler(store points.Store, admin bool, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := strconv.FormatInt(middleware.UserIDFromCtx(r.Context()), 10)
		if user == "0" {
			common.JsonError(w, common.ErrUnauthorized)
			return
		}
		page := 1
		if raw := r.URL.Query().Get("page"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 || n > 100000 {
				common.JsonError(w, common.ErrParam)
				return
			}
			page = n
		}
		var data interface{}
		var err error
		decode := func(v interface{}) error { return json.NewDecoder(http.MaxBytesReader(w, r.Body, 32768)).Decode(v) }
		pathID := func() (int64, error) {
			var p struct {
				ID int64 `path:"id"`
			}
			if err := httpx.ParsePath(r, &p); err != nil {
				return 0, err
			}
			if p.ID <= 0 {
				return 0, points.ErrInvalid
			}
			return p.ID, nil
		}
		switch action {
		case "account":
			data, err = store.Account(r.Context(), user)
		case "ledger":
			data, err = store.Entries(r.Context(), user, page)
		case "products":
			data, err = store.Products(r.Context(), admin, page, r.URL.Query().Get("keyword"), r.URL.Query().Get("status"))
		case "orders":
			data, err = store.Orders(r.Context(), user, admin, page, r.URL.Query().Get("keyword"), r.URL.Query().Get("status"))
		case "report":
			data, err = store.Report(r.Context())
		case "save":
			var p points.Product
			if decode(&p) != nil {
				err = points.ErrInvalid
				break
			}
			p.ID = 0
			if r.Method == http.MethodPut {
				p.ID, err = pathID()
				if err != nil {
					err = points.ErrInvalid
					break
				}
			}
			data, err = store.SaveProduct(r.Context(), p)
		case "redeem":
			var req points.RedeemRequest
			if decode(&req) != nil {
				err = points.ErrInvalid
				break
			}
			data, err = store.Redeem(r.Context(), user, req)
		default:
			var id int64
			id, err = pathID()
			if err != nil {
				err = points.ErrInvalid
				break
			}
			var req struct {
				Carrier    string `json:"carrier"`
				TrackingNo string `json:"trackingNo"`
			}
			if action == "ship" && decode(&req) != nil {
				err = points.ErrInvalid
				break
			}
			err = store.Transition(r.Context(), user, id, action, req.Carrier, req.TrackingNo, admin)
			data = map[string]bool{"success": err == nil}
		}
		if err != nil {
			switch {
			case errors.Is(err, points.ErrBalance), errors.Is(err, points.ErrConflict), errors.Is(err, points.ErrInvalid):
				common.JsonError(w, common.NewBizError(common.ErrParam.Code, err.Error()))
			case errors.Is(err, sqlx.ErrNotFound):
				common.JsonError(w, common.NewBizError(common.ErrParam.Code, "记录不存在"))
			default:
				logx.Errorf("points %s: %v", action, err)
				common.JsonError(w, common.ErrSystem)
			}
			return
		}
		common.Ok(w, data)
	}
}
