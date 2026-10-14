package handler

import (
	"github.com/askxuan/booking-service/internal/logic"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/rest/httpx"
	"net/http"
)

func bookingEligibilityHandler(s *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			MasterId string `form:"masterId"`
		}
		if httpx.Parse(r, &req) != nil {
			common.JsonError(w, common.ErrParamInvalid)
			return
		}
		result, err := logic.BookingEligibility(r.Context(), s, req.MasterId)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, result)
	}
}
