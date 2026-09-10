package handler

import (
	"net/http"
	"strconv"

	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/diy-service/internal/logic"
	"github.com/askxuan/diy-service/internal/svc"
	"github.com/askxuan/diy-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// RegisterHandlers 注册 diy 服务路由
func RegisterHandlers(server *rest.Server, svcCtx *svc.ServiceContext) {
	server.Use(middleware.CorsFunc)

	auth := &middleware.AuthConfig{Secret: svcCtx.Config.AuthSecret}
	customer := &middleware.AdminAuthConfig{AllowedRoles: []string{"customer"}}
	server.AddRoute(rest.Route{Method: http.MethodPut, Path: "/api/v1/diy/orders/:id/confirm", Handler: auth.AuthFunc(customer.AdminAuthFunc(diyOrderConfirmHandler(svcCtx)))})
	// ===== C端路由 =====
	server.AddRoutes([]rest.Route{
		{Method: http.MethodGet, Path: "/api/v1/diy/designs", Handler: designListHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/diy/my-designs", Handler: auth.AuthFunc(customer.AdminAuthFunc(myDesignListHandler(svcCtx)))},
		{Method: http.MethodPost, Path: "/api/v1/diy/designs", Handler: auth.AuthFunc(customer.AdminAuthFunc(designSaveHandler(svcCtx)))},
		{Method: http.MethodGet, Path: "/api/v1/diy/designs/:id", Handler: optionalDesignAuth(svcCtx, designDetailHandler(svcCtx))},
		{Method: http.MethodPost, Path: "/api/v1/diy/designs/:id/order", Handler: auth.AuthFunc(customer.AdminAuthFunc(diyDesignOrderCreateHandler(svcCtx)))},
		{Method: http.MethodGet, Path: "/api/v1/diy/materials", Handler: materialListHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/diy/blessing-services", Handler: blessingServiceListHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/diy/orders/availability", Handler: auth.AuthFunc(customer.AdminAuthFunc(diyOrderAvailabilityHandler(svcCtx)))},
		{Method: http.MethodPost, Path: "/api/v1/diy/orders", Handler: auth.AuthFunc(customer.AdminAuthFunc(diyOrderCreateHandler(svcCtx)))},
		{Method: http.MethodGet, Path: "/api/v1/diy/orders", Handler: auth.AuthFunc(customer.AdminAuthFunc(diyOrderListHandler(svcCtx)))},
		{Method: http.MethodGet, Path: "/api/v1/diy/orders/:id", Handler: auth.AuthFunc(customer.AdminAuthFunc(diyOrderDetailHandler(svcCtx)))},
	})

	server.AddRoutes([]rest.Route{
		{Method: http.MethodPost, Path: "/api/v1/diy/designs/:id/copy", Handler: auth.AuthFunc(customer.AdminAuthFunc(designStudioAction(svcCtx, "copy", false)))},
		{Method: http.MethodPut, Path: "/api/v1/diy/designs/:id/status", Handler: auth.AuthFunc(customer.AdminAuthFunc(designStudioAction(svcCtx, "status", false)))},
	})
	adminDesign := &middleware.AdminAuthConfig{AllowedRoles: []string{"shop_admin", "platform_super"}}
	server.AddRoutes([]rest.Route{
		{Method: http.MethodGet, Path: "/api/v1/admin/diy/designs", Handler: auth.AuthFunc(adminDesign.AdminAuthFunc(studioList(svcCtx)))},
		{Method: http.MethodPut, Path: "/api/v1/admin/diy/designs/:id/status", Handler: auth.AuthFunc(adminDesign.AdminAuthFunc(designStudioAction(svcCtx, "status", true)))},
	})
	// ===== 商城台路由 =====
	server.AddRoutes([]rest.Route{
		{Method: http.MethodGet, Path: "/api/v1/admin/diy/orders", Handler: adminDiyOrderListHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/diy/orders/:id", Handler: adminDiyOrderDetailHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/admin/diy/orders/:id/review", Handler: adminDiyOrderReviewHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/admin/diy/orders/:id/make-complete", Handler: adminDiyOrderMakeCompleteHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/admin/diy/orders/:id/ship", Handler: adminDiyOrderShipHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/diy/materials", Handler: adminMaterialListHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/diy/materials/:id", Handler: adminMaterialDetailHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/diy/materials", Handler: adminMaterialCreateHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/admin/diy/materials/:id", Handler: adminMaterialUpdateHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/admin/diy/materials/:id/status", Handler: adminMaterialStatusHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/diy/blessing-services", Handler: adminBlessingServiceListHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/diy/blessing-services/:id", Handler: adminBlessingServiceDetailHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/diy/blessing-services", Handler: adminBlessingServiceCreateHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/admin/diy/blessing-services/:id", Handler: adminBlessingServiceUpdateHandler(svcCtx)},
		{Method: http.MethodDelete, Path: "/api/v1/admin/diy/blessing-services/:id", Handler: adminBlessingServiceDeleteHandler(svcCtx)},
	})
}

// ===== C端 handler =====

// myDesignListHandler 我的设计列表（网关 JWT 后透传 X-User-Id）
func myDesignListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userId := ""
		if id := middleware.UserIDFromCtx(r.Context()); id > 0 {
			userId = strconv.FormatInt(id, 10)
		}
		if userId == "" {
			common.JsonError(w, common.ErrUnauthorized)
			return
		}
		page := 1
		size := 20
		if p := r.URL.Query().Get("page"); p != "" {
			if v, err := strconv.Atoi(p); err == nil {
				page = v
			}
		}
		if s := r.URL.Query().Get("size"); s != "" {
			if v, err := strconv.Atoi(s); err == nil {
				size = v
			}
		}
		resp, err := logic.NewMyDesignListLogic(r.Context(), svcCtx).List(userId, page, size)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func designListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DesignListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewDesignListLogic(r.Context(), svcCtx).List(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func designSaveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DesignSaveReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewDesignSaveLogic(r.Context(), svcCtx).Save(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func designDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DesignDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewDesignDetailLogic(r.Context(), svcCtx).Detail(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func diyDesignOrderCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DiyDesignOrderCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			logx.Errorf("diyDesignOrderCreate parse error: %v", err)
			common.JsonError(w, common.ErrParam)
			return
		}
		req.UserId = strconv.FormatInt(middleware.UserIDFromCtx(r.Context()), 10)
		resp, err := logic.NewDiyDesignOrderCreateLogic(r.Context(), svcCtx).Create(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func materialListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MaterialListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewMaterialListLogic(r.Context(), svcCtx).List(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func blessingServiceListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.BlessingServiceListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewBlessingServiceListLogic(r.Context(), svcCtx).List(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func diyOrderCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DiyOrderCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			logx.Errorf("diyOrderCreate parse error: %v", err)
			common.JsonError(w, common.ErrParam)
			return
		}
		req.UserId = strconv.FormatInt(middleware.UserIDFromCtx(r.Context()), 10)
		resp, err := logic.NewDiyOrderCreateLogic(r.Context(), svcCtx).Create(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func diyOrderAvailabilityHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DiyOrderAvailabilityReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewDiyOrderAvailabilityLogic(r.Context(), svcCtx).Check(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func diyOrderListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DiyOrderListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		req.UserId = strconv.FormatInt(middleware.UserIDFromCtx(r.Context()), 10)
		resp, err := logic.NewDiyOrderListLogic(r.Context(), svcCtx).List(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func diyOrderDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DiyOrderDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewDiyOrderDetailLogic(r.Context(), svcCtx).Detail(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

// ===== 商城台 handler =====

func adminDiyOrderListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminDiyOrderListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminDiyOrderListLogic(r.Context(), svcCtx).List(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminDiyOrderDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminDiyOrderDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminDiyOrderDetailLogic(r.Context(), svcCtx).Detail(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminDiyOrderReviewHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminDiyOrderReviewReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminDiyOrderReviewLogic(r.Context(), svcCtx).Review(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminDiyOrderMakeCompleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminDiyOrderMakeCompleteReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminDiyOrderMakeCompleteLogic(r.Context(), svcCtx).MakeComplete(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminDiyOrderShipHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminDiyOrderShipReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminDiyOrderShipLogic(r.Context(), svcCtx).Ship(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminMaterialListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminMaterialListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminMaterialListLogic(r.Context(), svcCtx).List(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminMaterialDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MaterialDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminMaterialDetailLogic(r.Context(), svcCtx).Detail(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminMaterialCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminMaterialCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminMaterialCreateLogic(r.Context(), svcCtx).Create(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminMaterialUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminMaterialUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminMaterialUpdateLogic(r.Context(), svcCtx).Update(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminMaterialStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminMaterialStatusReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminMaterialStatusLogic(r.Context(), svcCtx).Status(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminBlessingServiceListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminBlessingServiceListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminBlessingServiceListLogic(r.Context(), svcCtx).List(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminBlessingServiceDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminBlessingServiceDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminBlessingServiceDetailLogic(r.Context(), svcCtx).Detail(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminBlessingServiceCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminBlessingServiceCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminBlessingServiceCreateLogic(r.Context(), svcCtx).Create(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminBlessingServiceUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminBlessingServiceUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminBlessingServiceUpdateLogic(r.Context(), svcCtx).Update(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminBlessingServiceDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminBlessingServiceDeleteReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		if err := logic.NewAdminBlessingServiceDeleteLogic(r.Context(), svcCtx).Delete(&req); err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, nil)
		}
	}
}
