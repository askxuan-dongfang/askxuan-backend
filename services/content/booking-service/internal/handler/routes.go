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
	server.AddRoute(rest.Route{Method: http.MethodGet, Path: "/api/v1/consultations/quote", Handler: consultationQuoteHandler(svcCtx)})
	server.AddRoutes([]rest.Route{
		{Method: http.MethodPost, Path: "/openim/booking-chat-webhook", Handler: bookingChatWebhookHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/openim/booking-chat-webhook/:command", Handler: bookingChatWebhookHandler(svcCtx)},
	})

	// ============ C端分组（需JWT） ============
	server.AddRoutes(rest.WithMiddleware(authCfg.AuthFunc, []rest.Route{
		{Method: http.MethodGet, Path: "/api/v1/chats", Handler: chatListHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/chats/:id/messages", Handler: chatMessageListHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/chats/:id/messages", Handler: chatMessageSendHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/consultations", Handler: consultationCreateHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/consultations", Handler: consultationListHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/consultations/:id", Handler: consultationDetailHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/consultations/:id/pay", Handler: consultationPayHandler(svcCtx)},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/bookings/chats",
			Handler: chatListHandler(svcCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/api/v1/bookings",
			Handler: createHandler(svcCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/api/v1/master-bookings/:id",
			Handler: directBookingHandler(svcCtx),
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
			Method:  http.MethodGet,
			Path:    "/api/v1/bookings/:id/chat/messages",
			Handler: chatMessageListHandler(svcCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/api/v1/bookings/:id/chat/messages",
			Handler: chatMessageSendHandler(svcCtx),
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
	reportRoleCfg := &middleware.AdminAuthConfig{AllowedRoles: []string{"temple_admin", "platform_super"}}
	server.AddRoute(rest.Route{
		Method:  http.MethodGet,
		Path:    "/api/v1/admin/bookings/report",
		Handler: authCfg.AuthFunc(reportRoleCfg.AdminAuthFunc(adminBookingReportHandler(svcCtx))),
	})
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
			Path:    "/api/v1/admin/masters/bookings/:id/start",
			Handler: masterBookingStartHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/masters/bookings/:id/complete",
			Handler: masterBookingCompleteHandler(svcCtx),
		},
	}...))
}

func adminBookingReportHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminBookingReportReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminBookingReportLogic(r.Context(), svcCtx).Report(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
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

func consultationQuoteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ConsultationQuoteReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewConsultationLogic(r.Context(), svcCtx).Quote(&req)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

func consultationCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ConsultationCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewConsultationLogic(r.Context(), svcCtx).Create(&req)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

func consultationListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ConsultationListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewConsultationLogic(r.Context(), svcCtx).List(&req)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

func consultationDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ConsultationDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewConsultationLogic(r.Context(), svcCtx).Detail(&req)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

func consultationPayHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ConsultationPayReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewConsultationLogic(r.Context(), svcCtx).Pay(&req)
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

func chatListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ChatListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewChatListLogic(r.Context(), svcCtx).List(&req)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

func chatMessageListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ChatMessageListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewChatMessageListLogic(r.Context(), svcCtx).List(&req)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

func chatMessageSendHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ChatMessageSendReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewChatMessageSendLogic(r.Context(), svcCtx).Send(&req)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

// directBookingHandler 大师直约（先付费咨询后预约）
func directBookingHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DirectBookingReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewDirectBookingLogic(r.Context(), svcCtx).Create(req.MasterCode, &req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
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

func masterBookingStartHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminBookingActionReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewMasterBookingStartLogic(r.Context(), svcCtx)
		resp, err := l.MasterBookingStart(&req)
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
