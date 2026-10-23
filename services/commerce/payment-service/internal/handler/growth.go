package handler

import (
	"encoding/json"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/payment-service/internal/growth"
	"github.com/askxuan/payment-service/internal/svc"
	"github.com/zeromicro/go-zero/rest"
	"net/http"
	"strconv"
)

type growthRequest struct {
	ID           int64                `json:"id"`
	Revision     int64                `json:"revision"`
	SessionID    string               `json:"sessionId"`
	Visible      bool                 `json:"visible"`
	ReachedEnd   bool                 `json:"reachedEnd"`
	Action       string               `json:"action"`
	Body         string               `json:"body"`
	Day          string               `json:"day"`
	BadgeVisible bool                 `json:"badgeVisible"`
	RequestID    string               `json:"requestId"`
	Category     string               `json:"category"`
	Kind         string               `json:"kind"`
	Reason       string               `json:"reason"`
	Payload      json.RawMessage      `json:"payload"`
	Approve      bool                 `json:"approve"`
	Content      *growth.Content      `json:"content"`
	Activity     *growth.Activity     `json:"activity"`
	User         string               `json:"userId"`
	Evidence     string               `json:"evidence"`
	Reply        string               `json:"reply"`
	Urgency      int                  `json:"urgency"`
	ItemType     string               `json:"itemType"`
	ItemID       string               `json:"itemId"`
	Items        []growth.CatalogItem `json:"items"`
	Rule         *growth.RuleProposal `json:"rule"`
	Consent      bool                 `json:"consent"`
}

// Roles and the identity class must both come from the verified JWT. Customer
// and professional identities with the same numeric ID never share accounts.
func growthIdentity(r *http.Request, surface string) bool {
	t, _ := r.Context().Value(middleware.CtxKeyUserType).(string)
	has := func(role string) bool {
		for _, v := range middleware.RolesFromCtx(r.Context()) {
			if v == role {
				return true
			}
		}
		return false
	}
	if middleware.UserIDFromCtx(r.Context()) <= 0 {
		return false
	}
	switch surface {
	case "customer":
		return t == "user" && has("customer")
	case "admin":
		return t == "admin" && has("platform_super")
	case "provider":
		return t == "admin" && ((has("master") && middleware.MasterIDFromCtx(r.Context()) > 0) || (has("temple_admin") && middleware.TempleIDFromCtx(r.Context()) > 0 && middleware.TempleCodeFromCtx(r.Context()) != ""))
	}
	return false
}
func registerGrowth(server *rest.Server, s *svc.ServiceContext) {
	server.AddRoute(rest.Route{Method: "GET", Path: "/api/v1/payments/growth/public/rules", Handler: func(w http.ResponseWriter, r *http.Request) {
		if s.Growth == nil {
			common.JsonError(w, growth.ErrUnavailable)
			return
		}
		page := 1
		var e error
		if r.URL.Query().Get("page") != "" {
			page, e = strconv.Atoi(r.URL.Query().Get("page"))
		}
		if e != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		v, e := s.Growth.Rules(r.Context(), page, false)
		if e != nil {
			cashError(w, e)
			return
		}
		common.Ok(w, v)
	}})

	auth := &middleware.AuthConfig{Secret: s.Config.Auth.AccessSecret}
	add := func(method, path, surface string) {
		server.AddRoute(rest.Route{Method: method, Path: "/api/v1/payments/growth/" + path, Handler: auth.AuthFunc(func(w http.ResponseWriter, r *http.Request) {
			if !growthIdentity(r, surface) {
				common.JsonError(w, common.ErrRoleForbidden)
				return
			}
			if s.Growth == nil {
				common.JsonError(w, growth.ErrUnavailable)
				return
			}
			g := s.Growth
			ctx := r.Context()
			user := strconv.FormatInt(middleware.UserIDFromCtx(ctx), 10)
			if surface != "admin" && path != "rules" && path != "provider/rules" {
				cfg, e := g.Settings(ctx)
				if e != nil {
					cashError(w, e)
					return
				}
				if !cfg.Enabled {
					common.JsonError(w, growth.ErrUnavailable)
					return
				}
			}
			var req growthRequest
			if method != "GET" {
				if json.NewDecoder(http.MaxBytesReader(w, r.Body, 70000)).Decode(&req) != nil {
					common.JsonError(w, common.ErrParam)
					return
				}
			}
			var value any = map[string]bool{"ok": true}
			var err error
			q := r.URL.Query()
			switch path {
			case "rules", "provider/rules", "admin/rules":
				page := 1
				if q.Get("page") != "" {
					page, err = strconv.Atoi(q.Get("page"))
				}
				if err == nil {
					value, err = g.Rules(ctx, page, surface == "admin")
				}
			case "catalog/preview":
				value, err = g.CatalogPrice(ctx, user, req.Items)
			case "notice/claim":
				var n *growth.LevelNotice
				n, err = g.ClaimLevelNotice(ctx, user)
				value = map[string]any{"notice": n}
			case "admin/rules/preview":
				if req.Rule == nil {
					err = common.ErrParam
				} else {
					value, err = g.RulePreview(ctx, *req.Rule)
				}
			case "overview":
				value, err = g.Overview(ctx, user)
			case "records":
				page := 1
				if q.Get("page") != "" {
					page, err = strconv.Atoi(q.Get("page"))
				}
				if err == nil {
					value, err = g.Records(ctx, user, page)
				}
			case "profile":
				value, err = g.Profile(ctx, user)
			case "preferences":
				err = g.Preferences(ctx, user, req.BadgeVisible)
			case "notes":
				if method == "GET" {
					value, err = g.Notes(ctx, user)
				} else {
					err = g.SaveNote(ctx, user, req.Action, req.Body, false)
				}
			case "notes/delete":
				err = g.DeleteNote(ctx, user, req.Day)
			case "contents":
				value, err = g.Contents(ctx, false)
			case "learning/start":
				value, err = g.StartLearning(ctx, user, req.ID)
			case "learning/heartbeat":
				value, err = g.LearningHeartbeat(ctx, user, req.SessionID, req.Visible, req.ReachedEnd)
			case "activities":
				value, err = g.Activities(ctx, user, false)
			case "activities/register":
				err = g.RegisterActivity(ctx, user, req.ID)
			case "summary":
				value, err = g.Summary(ctx, user, q.Get("period"), q.Get("yearly") == "true")
			case "support":
				if method == "GET" {
					value, err = g.Support(ctx, user, false)
				} else {
					err = g.RequestSupport(ctx, user, req.RequestID, req.Category, req.Body)
				}
			case "order":
				value, err = g.OrderSnapshot(ctx, user, q.Get("orderType"), q.Get("orderNo"))
			case "admin/account":
				page := 1
				if q.Get("page") != "" {
					page, err = strconv.Atoi(q.Get("page"))
				}
				if err == nil {
					value, err = g.AccountAudit(ctx, q.Get("userId"), page)
				}
			case "admin/audit":
				value, err = g.Audit(ctx)
			case "admin/dashboard":
				value, err = g.Dashboard(ctx)
			case "admin/proposal":
				var id int64
				id, err = g.Propose(ctx, user, req.Kind, req.Reason, req.Payload)
				value = map[string]int64{"id": id}
			case "admin/approve":
				err = g.Approve(ctx, user, req.ID, req.Approve)
			case "admin/contents":
				if method == "GET" {
					value, err = g.Contents(ctx, true)
				} else if req.Content == nil {
					err = common.ErrParam
				} else {
					var id int64
					id, err = g.SaveContent(ctx, user, *req.Content)
					value = map[string]int64{"id": id}
				}
			case "admin/activities":
				if method == "GET" {
					value, err = g.Activities(ctx, "", true)
				} else if req.Activity == nil {
					err = common.ErrParam
				} else {
					var id int64
					id, err = g.SaveActivity(ctx, user, *req.Activity)
					value = map[string]int64{"id": id}
				}
			case "admin/attendance":
				err = g.VerifyAttendance(ctx, user, req.User, req.Evidence, req.ID)
			case "admin/support":
				if method == "GET" {
					value, err = g.Support(ctx, "", true)
				} else {
					err = g.ReplySupport(ctx, user, req.ID, req.Reply, req.Urgency)
				}
			case "provider/policies", "provider/bills", "provider/consent":
				kind := "temple"
				id := middleware.TempleIDFromCtx(ctx)
				code := middleware.TempleCodeFromCtx(ctx)
				if q.Get("ownerType") == "master" || id == 0 {
					kind = "master"
					id = middleware.MasterIDFromCtx(ctx)
					code = ""
				}
				var owner string
				owner, err = g.ProviderIdentity(ctx, kind, id, code)
				if err == nil {
					switch path {
					case "provider/policies":
						value, err = g.ProviderPolicies(ctx, kind, owner)
					case "provider/bills":
						value, err = g.ProviderBills(ctx, kind, owner)
					case "provider/consent":
						err = g.Consent(ctx, user, kind, owner, req.ItemType, req.ItemID, req.Consent, req.Revision)
					}
				}
			default:
				err = common.ErrParam
			}
			if err != nil {
				cashError(w, err)
				return
			}
			common.Ok(w, value)
		})})
	}
	for _, path := range []string{"overview", "records", "notes", "contents", "activities", "summary", "support", "order", "rules"} {
		add("GET", path, "customer")
	}
	for _, path := range []string{"profile", "preferences", "notes", "notes/delete", "learning/start", "learning/heartbeat", "activities/register", "support", "notice/claim", "catalog/preview"} {
		add("POST", path, "customer")
	}
	for _, path := range []string{"dashboard", "contents", "activities", "support", "audit", "account", "rules"} {
		add("GET", "admin/"+path, "admin")
	}
	for _, path := range []string{"proposal", "approve", "contents", "activities", "attendance", "support", "rules/preview"} {
		add("POST", "admin/"+path, "admin")
	}
	for _, path := range []string{"policies", "bills", "rules"} {
		add("GET", "provider/"+path, "provider")
	}
	add("POST", "provider/consent", "provider")
}
