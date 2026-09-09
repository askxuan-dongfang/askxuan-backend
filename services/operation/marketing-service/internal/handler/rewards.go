package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/marketing-service/internal/rewards"
	"github.com/askxuan/marketing-service/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func registerRewards(server *rest.Server, ctx *svc.ServiceContext) {
	auth := &middleware.AuthConfig{Secret: ctx.Config.Auth.AccessSecret}
	type operation struct {
		Method, Path, Action string
		Admin                bool
	}
	routes := []operation{
		{Method: http.MethodGet, Path: "/api/v1/marketing/rewards/campaigns", Action: "list", Admin: false},
		{Method: http.MethodGet, Path: "/api/v1/marketing/rewards/campaigns/:id", Action: "detail", Admin: false},
		{Method: http.MethodGet, Path: "/api/v1/marketing/rewards/orders", Action: "orders", Admin: false},
		{Method: http.MethodPost, Path: "/api/v1/marketing/rewards/campaigns/:id/join", Action: "join", Admin: false},
		{Method: http.MethodGet, Path: "/api/v1/marketing/rewards/entries", Action: "entries", Admin: false},
		{Method: http.MethodPost, Path: "/api/v1/marketing/rewards/orders/:id/claim", Action: "claim", Admin: false},
		{Method: http.MethodPost, Path: "/api/v1/marketing/rewards/orders/:id/complete", Action: "complete", Admin: false},
		{Method: http.MethodGet, Path: "/api/v1/admin/marketing/rewards/campaigns", Action: "list", Admin: true},
		{Method: http.MethodGet, Path: "/api/v1/admin/marketing/rewards/campaigns/:id", Action: "detail", Admin: true},
		{Method: http.MethodGet, Path: "/api/v1/admin/marketing/rewards/orders", Action: "orders", Admin: true},
		{Method: http.MethodPost, Path: "/api/v1/admin/marketing/rewards/campaigns", Action: "save", Admin: true},
		{Method: http.MethodPut, Path: "/api/v1/admin/marketing/rewards/campaigns/:id", Action: "save", Admin: true},
		{Method: http.MethodPost, Path: "/api/v1/admin/marketing/rewards/campaigns/:id/publish", Action: "publish", Admin: true},
		{Method: http.MethodPost, Path: "/api/v1/admin/marketing/rewards/campaigns/:id/cancel", Action: "cancel", Admin: true},
		{Method: http.MethodPost, Path: "/api/v1/admin/marketing/rewards/campaigns/:id/draw", Action: "draw", Admin: true},
		{Method: http.MethodGet, Path: "/api/v1/admin/marketing/rewards/campaigns/:id/audit", Action: "audit", Admin: true},
		{Method: http.MethodPost, Path: "/api/v1/admin/marketing/rewards/orders/:id/ship", Action: "ship", Admin: true},
	}
	for _, op := range routes {
		roles := []string{"customer"}
		if op.Admin {
			roles = []string{"platform_super"}
		}
		h := rewardHandler(ctx.Rewards, op.Admin, op.Action)
		h = (&middleware.AdminAuthConfig{AllowedRoles: roles}).AdminAuthFunc(h)
		server.AddRoute(rest.Route{Method: op.Method, Path: op.Path, Handler: auth.AuthFunc(h)})
	}
}
func rewardHandler(store rewards.Store, admin bool, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := strconv.FormatInt(middleware.UserIDFromCtx(r.Context()), 10)
		if user == "0" {
			common.JsonError(w, common.ErrUnauthorized)
			return
		}
		page := 1
		if raw := r.URL.Query().Get("page"); raw != "" {
			n, e := strconv.Atoi(raw)
			if e != nil || n < 1 || n > 100000 {
				common.JsonError(w, common.ErrParam)
				return
			}
			page = n
		}
		var path struct {
			ID int64 `path:"id"`
		}
		_ = httpx.ParsePath(r, &path)
		if action != "list" && action != "orders" && action != "entries" && !(action == "save" && r.Method == "POST") && path.ID < 1 {
			common.JsonError(w, common.ErrParam)
			return
		}
		decode := func(v any) error {
			d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 98304))
			d.DisallowUnknownFields()
			return d.Decode(v)
		}
		var data any = map[string]bool{"success": true}
		var e error
		switch action {
		case "list":
			data, e = store.List(r.Context(), admin, page, r.URL.Query().Get("kind"))
		case "detail":
			data, e = store.Detail(r.Context(), path.ID, user, admin)
		case "orders":
			data, e = store.Orders(r.Context(), user, admin, page, r.URL.Query().Get("status"))
		case "entries":
			data, e = store.Entries(r.Context(), user, page)
		case "save":
			var c rewards.Campaign
			if decode(&c) != nil {
				e = rewards.ErrInvalid
				break
			}
			c.ID = path.ID
			data, e = store.Save(r.Context(), c, user)
		case "join":
			data, e = store.Join(r.Context(), path.ID, user)
		case "publish":
			e = store.Publish(r.Context(), path.ID, user)
		case "cancel":
			var b struct {
				Reason string `json:"reason"`
			}
			if decode(&b) != nil {
				e = rewards.ErrInvalid
				break
			}
			e = store.Cancel(r.Context(), path.ID, user, b.Reason)
		case "draw":
			e = store.Draw(r.Context(), path.ID)
		case "audit":
			data, e = store.Audits(r.Context(), path.ID, page)
		case "claim", "ship", "complete":
			var a rewards.OrderAction
			if action != "complete" && decode(&a) != nil {
				e = rewards.ErrInvalid
				break
			}
			e = store.OrderAction(r.Context(), path.ID, user, action, a)
		}
		if e != nil {
			switch {
			case errors.Is(e, rewards.ErrInvalid), errors.Is(e, rewards.ErrClosed), errors.Is(e, rewards.ErrConflict), errors.Is(e, rewards.ErrNotFound):
				common.JsonError(w, common.NewBizError(40001, e.Error()))
			default:
				logx.Errorf("reward %s: %v", action, e)
				common.JsonError(w, common.ErrSystem)
			}
			return
		}
		common.Ok(w, data)
	}
}
