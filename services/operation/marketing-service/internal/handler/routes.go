package handler

import (
	"net/http"

	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/marketing-service/internal/logic"
	"github.com/askxuan/marketing-service/internal/svc"
	"github.com/askxuan/marketing-service/internal/types"

	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// RegisterHandlers 注册 marketing 服务路由
// C 端：/api/v1/marketing/*（公开，网关层不鉴权）
// 平台台：/api/v1/admin/marketing/*（网关层鉴权 + 角色校验）
func RegisterHandlers(server *rest.Server, svcCtx *svc.ServiceContext) {
	server.Use(middleware.CorsFunc)

	// ===== C 端 =====
	server.AddRoutes([]rest.Route{
		{Method: http.MethodGet, Path: "/api/v1/marketing/banners", Handler: customerBannerListHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/marketing/recommends", Handler: customerRecommendListHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/marketing/activities", Handler: customerActivityListHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/marketing/coupons", Handler: customerCouponListHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/marketing/coupons/:id/receive", Handler: customerCouponReceiveHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/marketing/my-coupons", Handler: customerMyCouponHandler(svcCtx)},
	})

	// ===== 平台管理台 =====
	server.AddRoutes([]rest.Route{
		{Method: http.MethodGet, Path: "/api/v1/admin/marketing/coupons", Handler: adminCouponListHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/marketing/coupons", Handler: adminCouponCreateHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/admin/marketing/coupons/:id", Handler: adminCouponUpdateHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/marketing/activities", Handler: adminActivityListHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/marketing/activities", Handler: adminActivityCreateHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/admin/marketing/activities/:id", Handler: adminActivityUpdateHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/marketing/banners", Handler: adminBannerListHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/marketing/banners", Handler: adminBannerCreateHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/admin/marketing/banners/:id", Handler: adminBannerUpdateHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/marketing/recommends", Handler: adminRecommendListHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/admin/marketing/recommends/:id", Handler: adminRecommendUpdateHandler(svcCtx)},
	})
}

// ===== C 端 handlers =====

func customerBannerListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.BannerListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewCustomerBannerListLogic(r.Context(), svcCtx).BannerList(&req)
		respond(w, resp, err)
	}
}

func customerRecommendListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.RecommendListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewCustomerRecommendListLogic(r.Context(), svcCtx).RecommendList(&req)
		respond(w, resp, err)
	}
}

func customerActivityListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ActivityListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewCustomerActivityListLogic(r.Context(), svcCtx).ActivityList(&req)
		respond(w, resp, err)
	}
}

func customerCouponListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CouponListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewCustomerCouponListLogic(r.Context(), svcCtx).CouponList(&req)
		respond(w, resp, err)
	}
}

func customerCouponReceiveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CouponReceiveReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		if userId := r.Header.Get("X-User-Id"); userId != "" {
			req.UserId = userId
		}
		resp, err := logic.NewCustomerCouponReceiveLogic(r.Context(), svcCtx).Receive(&req)
		respond(w, resp, err)
	}
}

func customerMyCouponHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MyCouponReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		if userId := r.Header.Get("X-User-Id"); userId != "" {
			req.UserId = userId
		}
		resp, err := logic.NewCustomerMyCouponLogic(r.Context(), svcCtx).MyCoupon(&req)
		respond(w, resp, err)
	}
}

// ===== 平台台 handlers =====

func adminCouponListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CouponListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminCouponListLogic(r.Context(), svcCtx).List(&req)
		respond(w, resp, err)
	}
}

func adminCouponCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CouponCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminCouponCreateLogic(r.Context(), svcCtx).Create(&req)
		respond(w, resp, err)
	}
}

func adminCouponUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CouponUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminCouponUpdateLogic(r.Context(), svcCtx).Update(&req)
		respond(w, resp, err)
	}
}

func adminActivityListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ActivityListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminActivityListLogic(r.Context(), svcCtx).List(&req)
		respond(w, resp, err)
	}
}

func adminActivityCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ActivityCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminActivityCreateLogic(r.Context(), svcCtx).Create(&req)
		respond(w, resp, err)
	}
}

func adminActivityUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ActivityUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminActivityUpdateLogic(r.Context(), svcCtx).Update(&req)
		respond(w, resp, err)
	}
}

func adminBannerListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.BannerListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminBannerListLogic(r.Context(), svcCtx).List(&req)
		respond(w, resp, err)
	}
}

func adminBannerCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.BannerCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminBannerCreateLogic(r.Context(), svcCtx).Create(&req)
		respond(w, resp, err)
	}
}

func adminBannerUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.BannerUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminBannerUpdateLogic(r.Context(), svcCtx).Update(&req)
		respond(w, resp, err)
	}
}

func adminRecommendListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.RecommendListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminRecommendListLogic(r.Context(), svcCtx).List(&req)
		respond(w, resp, err)
	}
}

func adminRecommendUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.RecommendUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminRecommendUpdateLogic(r.Context(), svcCtx).Update(&req)
		respond(w, resp, err)
	}
}

// respond 统一响应封装
func respond(w http.ResponseWriter, resp interface{}, err error) {
	if err != nil {
		common.JsonError(w, err)
	} else {
		common.Ok(w, resp)
	}
}
