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
)

// RegisterHandlers 注册 booking 服务路由
func RegisterHandlers(server *rest.Server, svcCtx *svc.ServiceContext) {
	server.Use(middleware.CorsFunc)

	// JWT 鉴权配置（法师工作台接口需要登录）
	authCfg := &middleware.AuthConfig{Secret: svcCtx.Config.AuthSecret}

	// 可用时段无需登录，金额与剩余容量均由服务端返回。
	server.AddRoute(rest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/bookings/availability",
		Handler: availabilityHandler(svcCtx),
	})

	// ============ C端分组（需JWT） ============
	server.AddRoutes(rest.WithMiddleware(authCfg.AuthFunc, []rest.Route{
		{
			Method:  http.MethodPost,
			Path:    "/api/v1/bookings",
			Handler: createHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/bookings",
			Handler: listHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/bookings/:id",
			Handler: detailHandler(svcCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/api/v1/bookings/:id/pay",
			Handler: payHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/bookings/:id/status",
			Handler: updateStatusHandler(svcCtx),
		},
		// 评价
		{
			Method:  http.MethodPost,
			Path:    "/api/v1/bookings/:id/review",
			Handler: createReviewHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/bookings/:id/review",
			Handler: reviewDetailHandler(svcCtx),
		},
	}...))

	// ============ 寺院管理台分组 ============
	server.AddRoutes([]rest.Route{
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/bookings",
			Handler: adminBookingListHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/bookings/:id",
			Handler: adminBookingDetailHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/bookings/:id/confirm",
			Handler: adminBookingConfirmHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/bookings/:id/complete",
			Handler: adminBookingCompleteHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/bookings/:id/cancel",
			Handler: adminBookingCancelHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/bookings/:id/timeout-cancel",
			Handler: adminBookingTimeoutCancelHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/bookings/:id/status-log",
			Handler: adminBookingStatusLogHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/bookings/:id/review/reply",
			Handler: adminReviewReplyHandler(svcCtx),
		},
	})

	// ============ 法师工作台分组（需JWT） ============
	server.AddRoutes(rest.WithMiddleware(authCfg.AuthFunc, []rest.Route{
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/masters/bookings",
			Handler: masterBookingListHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/masters/bookings/:id",
			Handler: masterBookingDetailHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/masters/bookings/:id/confirm",
			Handler: masterBookingConfirmHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/masters/bookings/:id/complete",
			Handler: masterBookingCompleteHandler(svcCtx),
		},
	}...))
}

// ============ C端 Handler ============

func availabilityHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AvailabilityReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAvailabilityLogic(r.Context(), svcCtx).Availability(&req)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

func payHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.PayReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewPayLogic(r.Context(), svcCtx).Pay(&req)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

func createHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CreateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewCreateLogic(r.Context(), svcCtx)
		resp, err := l.Create(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func listHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewListLogic(r.Context(), svcCtx)
		resp, err := l.List(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func detailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewDetailLogic(r.Context(), svcCtx)
		resp, err := l.Detail(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func updateStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.StatusReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewUpdateStatusLogic(r.Context(), svcCtx)
		resp, err := l.UpdateStatus(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func createReviewHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ReviewCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewCreateReviewLogic(r.Context(), svcCtx)
		resp, err := l.CreateReview(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func reviewDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ReviewDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewReviewDetailLogic(r.Context(), svcCtx)
		resp, err := l.ReviewDetail(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

// ============ 寺院管理台 Handler ============

func adminBookingListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminBookingListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminBookingListLogic(r.Context(), svcCtx)
		resp, err := l.AdminBookingList(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminBookingDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminBookingDetailLogic(r.Context(), svcCtx)
		resp, err := l.AdminBookingDetail(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminBookingConfirmHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminBookingActionReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminBookingConfirmLogic(r.Context(), svcCtx)
		resp, err := l.AdminBookingConfirm(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminBookingCompleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminBookingActionReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminBookingCompleteLogic(r.Context(), svcCtx)
		resp, err := l.AdminBookingComplete(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminBookingCancelHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminBookingActionReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminBookingCancelLogic(r.Context(), svcCtx)
		resp, err := l.AdminBookingCancel(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminBookingTimeoutCancelHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminBookingActionReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminBookingTimeoutCancelLogic(r.Context(), svcCtx)
		resp, err := l.AdminBookingTimeoutCancel(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminBookingStatusLogHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.StatusLogReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminBookingStatusLogLogic(r.Context(), svcCtx)
		resp, err := l.AdminBookingStatusLog(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminReviewReplyHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ReviewReplyReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminReviewReplyLogic(r.Context(), svcCtx)
		resp, err := l.AdminReviewReply(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

// ============ 法师工作台 Handler ============

func masterBookingListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MasterBookingListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewMasterBookingListLogic(r.Context(), svcCtx)
		resp, err := l.MasterBookingList(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func masterBookingDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewMasterBookingDetailLogic(r.Context(), svcCtx)
		resp, err := l.MasterBookingDetail(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func masterBookingConfirmHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminBookingActionReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewMasterBookingConfirmLogic(r.Context(), svcCtx)
		resp, err := l.MasterBookingConfirm(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func masterBookingCompleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminBookingActionReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewMasterBookingCompleteLogic(r.Context(), svcCtx)
		resp, err := l.MasterBookingComplete(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}
