package handler

import (
	"net/http"

	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/order-service/internal/logic"
	"github.com/askxuan/order-service/internal/svc"
	"github.com/askxuan/order-service/internal/types"

	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// RegisterHandlers 注册 order 服务路由
func RegisterHandlers(server *rest.Server, svcCtx *svc.ServiceContext) {
	server.Use(middleware.CorsFunc)
	authCfg := &middleware.AuthConfig{Secret: svcCtx.Config.AuthSecret}

	// ===== C端路由 =====
	server.AddRoutes(rest.WithMiddleware(authCfg.AuthFunc, []rest.Route{
		{Method: http.MethodPost, Path: "/api/v1/orders", Handler: orderCreateHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/orders", Handler: orderListHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/orders/:id", Handler: orderDetailHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/orders/:id/confirm", Handler: orderConfirmHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/orders/:id/return", Handler: orderReturnHandler(svcCtx)},
	}...))

	// ===== 商城台路由 =====
	server.AddRoutes([]rest.Route{
		{Method: http.MethodGet, Path: "/api/v1/admin/orders", Handler: adminOrderListHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/orders/:id", Handler: adminOrderDetailHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/admin/orders/:id/ship", Handler: adminOrderShipHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/orders/returns", Handler: adminReturnListHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/admin/orders/returns/:id/review", Handler: adminReturnReviewHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/admin/orders/returns/:id/refund", Handler: adminReturnRefundHandler(svcCtx)},
	})
}

// ===== C端 handler =====

func orderCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.OrderCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewOrderCreateLogic(r.Context(), svcCtx).Create(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func orderListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.OrderListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewOrderListLogic(r.Context(), svcCtx).List(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func orderDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.OrderDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewOrderDetailLogic(r.Context(), svcCtx).Detail(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func orderConfirmHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.OrderConfirmReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewOrderConfirmLogic(r.Context(), svcCtx).Confirm(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func orderReturnHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.OrderReturnReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewOrderReturnLogic(r.Context(), svcCtx).Return(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

// ===== 商城台 handler =====

func adminOrderListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminOrderListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminOrderListLogic(r.Context(), svcCtx).List(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminOrderDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminOrderDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminOrderDetailLogic(r.Context(), svcCtx).Detail(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminOrderShipHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminOrderShipReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminOrderShipLogic(r.Context(), svcCtx).Ship(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminReturnListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminReturnListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminReturnListLogic(r.Context(), svcCtx).List(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminReturnReviewHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminReturnReviewReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminReturnReviewLogic(r.Context(), svcCtx).Review(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminReturnRefundHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminReturnRefundReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminReturnRefundLogic(r.Context(), svcCtx).Refund(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}
