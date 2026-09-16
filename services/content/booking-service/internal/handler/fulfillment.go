package handler

import (
	"github.com/askxuan/booking-service/internal/logic"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/booking-service/internal/types"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"

	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"net/http"
	"strconv"
)

func templeBookingGuard(s *svc.ServiceContext) rest.Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			allowed := false
			for _, role := range middleware.RolesFromCtx(r.Context()) {
				if role == "temple_admin" && middleware.TempleCodeFromCtx(r.Context()) == "" {
					common.JsonError(w, common.ErrTempleIsolation)
					return
				}
				if role == "temple_admin" || role == "platform_super" {
					allowed = true
				}
			}
			if !allowed {
				common.JsonError(w, common.ErrForbidden)
				return
			}
			if id := pathvar.Vars(r)["id"]; id != "" {
				if _, _, _, err := logic.BookingAccess(r.Context(), s, id, false); err != nil {
					common.JsonError(w, err)
					return
				}
			} else if code := middleware.TempleCodeFromCtx(r.Context()); code != "" && r.URL.Query().Get("templeId") != code {
				common.JsonError(w, common.ErrTempleIsolation)
				return
			}
			next(w, r)
		}
	}
}
func registerFulfillment(server *rest.Server, s *svc.ServiceContext, auth *middleware.AuthConfig) {
	route := func(method, path string, handler http.HandlerFunc) {
		server.AddRoute(rest.Route{Method: method, Path: path, Handler: auth.AuthFunc(handler)}, rest.WithMaxBytes(21<<20))
	}
	route(http.MethodGet, "/api/v1/bookings/journeys", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		page := 1
		var err error
		if q.Get("page") != "" {
			page, err = strconv.Atoi(q.Get("page"))
		}
		if err != nil {
			common.JsonError(w, common.ErrParamInvalid)
			return
		}
		filter := q.Get("filter")
		if filter == "" {
			filter = "all"
		}
		resp, err := logic.JourneyList(r.Context(), s, page, filter, q.Get("q"), q.Get("from"), q.Get("to"))
		replyFulfillment(w, resp, err)
	})
	for _, kind := range []string{"wish", "update"} {
		kind := kind
		route(http.MethodPost, "/api/v1/bookings/:id/progress/"+kind, func(w http.ResponseWriter, r *http.Request) {
			var req logic.ProgressRequest
			if httpx.Parse(r, &req) != nil {
				common.JsonError(w, common.ErrParamInvalid)
				return
			}
			resp, err := logic.PublishProgress(r.Context(), s, pathvar.Vars(r)["id"], kind, req)
			replyFulfillment(w, resp, err)
		})
	}
	route(http.MethodGet, "/api/v1/bookings/:id/fulfillment", func(w http.ResponseWriter, r *http.Request) {
		resp, err := logic.Fulfillment(r.Context(), s, pathvar.Vars(r)["id"])
		replyFulfillment(w, resp, err)
	})
	route(http.MethodPost, "/api/v1/bookings/:id/receipts", func(w http.ResponseWriter, r *http.Request) {
		var req logic.ReceiptRequest
		if httpx.Parse(r, &req) != nil {
			common.JsonError(w, common.ErrParamInvalid)
			return
		}
		resp, err := logic.SubmitReceipt(r.Context(), s, pathvar.Vars(r)["id"], req)
		replyFulfillment(w, resp, err)
	})
	for _, accept := range []bool{true, false} {
		accept := accept
		action := "request-revision"
		if accept {
			action = "accept-receipt"
		}
		route(http.MethodPost, "/api/v1/bookings/:id/"+action, func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Remark string `json:"remark,optional"`
			}
			if httpx.Parse(r, &req) != nil {
				common.JsonError(w, common.ErrParamInvalid)
				return
			}
			resp, err := logic.ReceiptDecision(r.Context(), s, pathvar.Vars(r)["id"], req.Remark, accept)
			replyFulfillment(w, resp, err)
		})
	}
	route(http.MethodPost, "/api/v1/bookings/:id/receipt-files", func(w http.ResponseWriter, r *http.Request) { logic.ReceiptUpload(w, r, s, pathvar.Vars(r)["id"]) })
	route(http.MethodGet, "/api/v1/bookings/:id/receipt-files/:file", func(w http.ResponseWriter, r *http.Request) {
		v := pathvar.Vars(r)
		logic.ReceiptDownload(w, r, s, v["id"], v["file"])
	})
	route(http.MethodPut, "/api/v1/admin/bookings/:id/start", templeBookingGuard(s)(func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminBookingActionReq
		if httpx.Parse(r, &req) != nil {
			common.JsonError(w, common.ErrParamInvalid)
			return
		}
		// The existing transition entry applies the same ownership and atomic audit.
		resp, err := logic.StartTempleBooking(r.Context(), s, req.Id, req.Remark)
		replyFulfillment(w, resp, err)
	}))
}
func replyFulfillment(w http.ResponseWriter, resp any, err error) {
	if err != nil {
		common.JsonError(w, err)
	} else {
		common.Ok(w, resp)
	}
}
