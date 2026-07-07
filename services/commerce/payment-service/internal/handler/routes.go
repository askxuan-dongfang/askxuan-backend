package handler

import (
	"net/http"

	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/payment-service/internal/logic"
	"github.com/askxuan/payment-service/internal/svc"
	"github.com/askxuan/payment-service/internal/types"

	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// RegisterHandlers 注册 payment 服务路由
func RegisterHandlers(server *rest.Server, svcCtx *svc.ServiceContext) {
	server.Use(middleware.CorsFunc)

	// ===== C端路由 =====
	server.AddRoutes([]rest.Route{
		{Method: http.MethodPost, Path: "/api/v1/payments", Handler: paymentCreateHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/payments/:id", Handler: paymentQueryHandler(svcCtx)},
	})

	// ===== 回调路由（公开，无需鉴权） =====
	server.AddRoutes([]rest.Route{
		{Method: http.MethodPost, Path: "/api/v1/payments/callback/wechat", Handler: callbackWechatHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/payments/callback/alipay", Handler: callbackAlipayHandler(svcCtx)},
	})

	// ===== 退款路由（服务侧 JWT 鉴权 + 网关层 JWT 校验，纵深防御） =====
	server.AddRoutes([]rest.Route{
		{Method: http.MethodPost, Path: "/api/v1/payments/refund", Handler: refundHandler(svcCtx)},
	}, rest.WithJwt(svcCtx.Config.Auth.AccessSecret))
}

func paymentCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.PaymentCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewPaymentCreateLogic(r.Context(), svcCtx).Create(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func paymentQueryHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.PaymentQueryReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewPaymentQueryLogic(r.Context(), svcCtx).Query(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func callbackWechatHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CallbackWechatReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewCallbackWechatLogic(r.Context(), svcCtx).Callback(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func callbackAlipayHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CallbackAlipayReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewCallbackAlipayLogic(r.Context(), svcCtx).Callback(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func refundHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.RefundReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewRefundLogic(r.Context(), svcCtx).Refund(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}
