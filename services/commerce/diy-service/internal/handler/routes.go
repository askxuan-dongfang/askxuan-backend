package handler

import (
	"net/http"

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

	// ===== C端路由 =====
	server.AddRoutes([]rest.Route{
		{Method: http.MethodGet, Path: "/api/v1/diy/designs", Handler: designListHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/diy/designs", Handler: designSaveHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/diy/designs/:id", Handler: designDetailHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/diy/designs/:id/order", Handler: diyDesignOrderCreateHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/diy/materials", Handler: materialListHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/diy/blessing-services", Handler: blessingServiceListHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/diy/orders", Handler: diyOrderCreateHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/diy/orders", Handler: diyOrderListHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/diy/orders/:id", Handler: diyOrderDetailHandler(svcCtx)},
	})

	// ===== 商城台路由 =====
	server.AddRoutes([]rest.Route{
		{Method: http.MethodGet, Path: "/api/v1/admin/diy/orders", Handler: adminDiyOrderListHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/diy/orders/:id", Handler: adminDiyOrderDetailHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/admin/diy/orders/:id/review", Handler: adminDiyOrderReviewHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/admin/diy/orders/:id/make-complete", Handler: adminDiyOrderMakeCompleteHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/admin/diy/orders/:id/ship", Handler: adminDiyOrderShipHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/diy/materials", Handler: adminMaterialListHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/diy/materials", Handler: adminMaterialCreateHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/admin/diy/materials/:id", Handler: adminMaterialUpdateHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/admin/diy/materials/:id/status", Handler: adminMaterialStatusHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/diy/blessing-services", Handler: adminBlessingServiceListHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/diy/blessing-services", Handler: adminBlessingServiceCreateHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/admin/diy/blessing-services/:id", Handler: adminBlessingServiceUpdateHandler(svcCtx)},
		{Method: http.MethodDelete, Path: "/api/v1/admin/diy/blessing-services/:id", Handler: adminBlessingServiceDeleteHandler(svcCtx)},
	})
}

// ===== C端 handler =====

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
		resp, err := logic.NewDiyOrderCreateLogic(r.Context(), svcCtx).Create(&req)
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
