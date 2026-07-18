package handler

import (
	"net/http"

	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/review-service/internal/logic"
	"github.com/askxuan/review-service/internal/svc"
	"github.com/askxuan/review-service/internal/types"

	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// RegisterHandlers 注册 review 服务路由
func RegisterHandlers(server *rest.Server, svcCtx *svc.ServiceContext) {
	server.Use(middleware.CorsFunc)

	// C端评价接口
	server.AddRoutes([]rest.Route{
		{
			Method:  http.MethodPost,
			Path:    "/api/v1/reviews",
			Handler: createReviewHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/reviews",
			Handler: reviewListHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/reviews/:id",
			Handler: reviewDetailHandler(svcCtx),
		},
	})

	// 寺院台/法师台评价管理接口
	server.AddRoutes([]rest.Route{
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/reviews",
			Handler: adminReviewListHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/reviews/:id",
			Handler: adminReviewDetailHandler(svcCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/api/v1/admin/reviews/:id/reply",
			Handler: reviewReplyHandler(svcCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/api/v1/admin/reviews/:id/report",
			Handler: reviewReportHandler(svcCtx),
		},
	})

	// 平台台举报处理接口
	server.AddRoutes([]rest.Route{
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/platform/reviews/reports",
			Handler: reportListHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/platform/reviews/reports/:id/handle",
			Handler: reportHandleHandler(svcCtx),
		},
	})

	// 法师台评价管理接口（JWT 鉴权，masterId 从 JWT 获取）
	server.AddRoutes(rest.WithMiddleware(svcCtx.AuthConfig.AuthFunc, []rest.Route{
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/masters/reviews",
			Handler: masterReviewListHandler(svcCtx),
		},
	}...))
}

func createReviewHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CreateReviewReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		if userId := r.Header.Get("X-User-Id"); userId != "" {
			req.UserId = userId
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

func reviewListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ReviewListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewReviewListLogic(r.Context(), svcCtx)
		resp, err := l.ReviewList(&req)
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

func adminReviewListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminReviewListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminReviewListLogic(r.Context(), svcCtx)
		resp, err := l.AdminReviewList(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminReviewDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ReviewDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminReviewDetailLogic(r.Context(), svcCtx)
		resp, err := l.AdminReviewDetail(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func reviewReplyHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ReviewReplyReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewReviewReplyLogic(r.Context(), svcCtx)
		resp, err := l.ReviewReply(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func reviewReportHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ReviewReportReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewReviewReportLogic(r.Context(), svcCtx)
		resp, err := l.ReviewReport(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func reportListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ReportListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewReportListLogic(r.Context(), svcCtx)
		resp, err := l.ReportList(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func reportHandleHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ReportHandleReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewReportHandleLogic(r.Context(), svcCtx)
		resp, err := l.ReportHandle(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func masterReviewListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MasterReviewListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewMasterReviewListLogic(r.Context(), svcCtx)
		resp, err := l.MasterReviewList(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}
