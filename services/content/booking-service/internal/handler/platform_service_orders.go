package handler

import (
	"net/http"

	"github.com/askxuan/booking-service/internal/logic"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/booking-service/internal/types"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
	"github.com/zeromicro/go-zero/rest/pathvar"
)

func registerPlatformServiceOrders(server *rest.Server, s *svc.ServiceContext, auth *middleware.AuthConfig) {
	const prefix = "/api/v1/admin/platform/service-orders"
	for _, route := range []struct {
		path, kind string
	}{
		{"/bookings", "bookings"}, {"/bookings/:id", "booking"},
		{"/bookings/:id/fulfillment", "fulfillment"},
		{"/consultations", "consultations"}, {"/consultations/:id", "consultation"},
	} {
		server.AddRoute(rest.Route{Method: http.MethodGet, Path: prefix + route.path,
			Handler: auth.AuthFunc(platformServiceOrdersHandler(s, route.kind))})
	}
}

func platformServiceOrdersHandler(s *svc.ServiceContext, kind string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := logic.RequirePlatformServiceReader(r.Context()); err != nil {
			common.JsonError(w, err)
			return
		}
		l := logic.NewPlatformServiceOrdersLogic(r.Context(), s)
		var response any
		var err error
		if kind == "bookings" || kind == "consultations" {
			var req types.PlatformServiceOrderListReq
			if parseErr := httpx.Parse(r, &req); parseErr != nil {
				common.JsonError(w, common.ErrParamInvalid)
				return
			}
			if kind == "bookings" {
				response, err = l.Bookings(&req)
			} else {
				response, err = l.Consultations(&req)
			}
		} else {
			id := pathvar.Vars(r)["id"]
			switch kind {
			case "booking":
				response, err = l.Booking(id)
			case "consultation":
				response, err = l.Consultation(id)
			case "fulfillment":
				response, err = l.Fulfillment(id)
			default:
				err = common.ErrParamInvalid
			}
		}
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, response)
	}
}
