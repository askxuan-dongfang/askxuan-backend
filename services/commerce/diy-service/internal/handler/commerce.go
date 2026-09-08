package handler

import (
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/diy-service/internal/model"
	"github.com/askxuan/diy-service/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
	"net/http"
	"strconv"
)

func diyOrderConfirmHandler(s *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID int64 `path:"id"`
		}
		if httpx.ParsePath(r, &req) != nil || req.ID < 1 {
			common.JsonError(w, common.ErrParam)
			return
		}
		o, err := s.DiyOrderModel.FindOne(r.Context(), req.ID)
		if err != nil {
			common.JsonError(w, common.ErrDiyOrderNotFound)
			return
		}
		if o.UserId != strconv.FormatInt(middleware.UserIDFromCtx(r.Context()), 10) {
			common.JsonError(w, common.ErrForbidden)
			return
		}
		if o.Status == model.DiyStatusCompleted {
			common.Ok(w, o)
			return
		}
		if o.Status != model.DiyStatusShipped {
			common.JsonError(w, common.ErrStatusInvalid)
			return
		}
		updated, err := s.DiyOrderModel.UpdateStatusIfCurrent(r.Context(), o.Id, model.DiyStatusShipped, model.DiyStatusCompleted)
		if err != nil {
			common.JsonError(w, common.ErrStatusInvalid)
			return
		}
		common.Ok(w, updated)
	}
}
