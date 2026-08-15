package handler

import (
	"net/http"
	"strconv"

	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/product-service/internal/logic"
	"github.com/askxuan/product-service/internal/svc"
	"github.com/askxuan/product-service/internal/types"

	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// RegisterHandlers 注册 product 服务路由
func RegisterHandlers(server *rest.Server, svcCtx *svc.ServiceContext) {
	server.Use(middleware.CorsFunc)

	// ===== C端路由（只读） =====
	server.AddRoutes([]rest.Route{
		{Method: http.MethodGet, Path: "/api/v1/products", Handler: customerProductListHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/products/:id", Handler: customerProductDetailHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/products/categories", Handler: customerCategoryTreeHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/intentions", Handler: customerIntentionHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/intentions/tags", Handler: customerIntentionTagListHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/products/:id/favorite", Handler: productFavoriteHandler(svcCtx, true)},
		{Method: http.MethodDelete, Path: "/api/v1/products/:id/favorite", Handler: productFavoriteHandler(svcCtx, false)},
		{Method: http.MethodGet, Path: "/api/v1/favorites/products", Handler: productFavoritesHandler(svcCtx)},
	})

	// ===== 商城台路由 =====
	server.AddRoutes([]rest.Route{
		{Method: http.MethodGet, Path: "/api/v1/admin/products", Handler: adminProductListHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/products", Handler: adminProductCreateHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/products/:id", Handler: adminProductDetailHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/admin/products/:id", Handler: adminProductUpdateHandler(svcCtx)},
		{Method: http.MethodDelete, Path: "/api/v1/admin/products/:id", Handler: adminProductDeleteHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/admin/products/:id/status", Handler: adminProductStatusHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/products/:id/skus", Handler: adminSkuCreateHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/admin/products/:id/skus/:skuId", Handler: adminSkuUpdateHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/products/categories", Handler: adminCategoryListHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/products/categories", Handler: adminCategoryCreateHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/admin/products/categories/:id", Handler: adminCategoryUpdateHandler(svcCtx)},
		{Method: http.MethodDelete, Path: "/api/v1/admin/products/categories/:id", Handler: adminCategoryDeleteHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/platform/intentions", Handler: adminIntentionTagListHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/platform/intentions", Handler: adminIntentionTagCreateHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/admin/platform/intentions/:code", Handler: adminIntentionTagUpdateHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/admin/platform/intentions/:code/status", Handler: adminIntentionTagStatusHandler(svcCtx)},
	})
}

func customerIntentionTagListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := logic.ListIntentionTags(r.Context(), svcCtx, false)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

func adminIntentionTagListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := logic.ListIntentionTags(r.Context(), svcCtx, true)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

func adminIntentionTagCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminIntentionTagCreateReq
		if httpx.Parse(r, &req) != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.CreateIntentionTag(r.Context(), svcCtx, &req)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

func adminIntentionTagUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminIntentionTagUpdateReq
		if httpx.Parse(r, &req) != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.UpdateIntentionTag(r.Context(), svcCtx, &req)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

func adminIntentionTagStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminIntentionTagStatusReq
		if httpx.Parse(r, &req) != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		if err := logic.UpdateIntentionTagStatus(r.Context(), svcCtx, &req); err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, req)
	}
}

func customerIntentionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CustomerIntentionReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewCustomerIntentionLogic(r.Context(), svcCtx).List(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

// ===== C端 handler =====

func customerProductListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CustomerProductListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewCustomerProductListLogic(r.Context(), svcCtx).List(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func customerProductDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CustomerProductDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewCustomerProductDetailLogic(r.Context(), svcCtx).Detail(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func customerCategoryTreeHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CustomerCategoryTreeReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewCustomerCategoryTreeLogic(r.Context(), svcCtx).Tree(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

// productFavoriteHandler 收藏/取消收藏商品（JWT 由网关校验，服务内取 X-User-Id）
func productFavoriteHandler(svcCtx *svc.ServiceContext, favorited bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userId, err := strconv.ParseInt(r.Header.Get("X-User-Id"), 10, 64)
		if err != nil || userId <= 0 {
			common.JsonError(w, common.ErrUnauthorized)
			return
		}
		var req types.ProductFavoriteReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.SetProductFavorite(r.Context(), svcCtx, userId, req.Id, favorited)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

// productFavoritesHandler 查询用户收藏的商品列表
func productFavoritesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userId, err := strconv.ParseInt(r.Header.Get("X-User-Id"), 10, 64)
		if err != nil || userId <= 0 {
			common.JsonError(w, common.ErrUnauthorized)
			return
		}
		resp, err := logic.ListFavoriteProducts(r.Context(), svcCtx, userId)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

// ===== 商城台 handler =====

func adminProductListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminProductListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminProductListLogic(r.Context(), svcCtx).List(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminProductCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminProductCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminProductCreateLogic(r.Context(), svcCtx).Create(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminProductDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminProductDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminProductDetailLogic(r.Context(), svcCtx).Detail(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminProductUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminProductUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminProductUpdateLogic(r.Context(), svcCtx).Update(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminProductDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminProductDeleteReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		err := logic.NewAdminProductDeleteLogic(r.Context(), svcCtx).Delete(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, nil)
		}
	}
}

func adminProductStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminProductStatusReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminProductStatusLogic(r.Context(), svcCtx).Status(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminSkuCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminSkuCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminSkuCreateLogic(r.Context(), svcCtx).Create(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminSkuUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminSkuUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminSkuUpdateLogic(r.Context(), svcCtx).Update(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminCategoryListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminCategoryListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminCategoryListLogic(r.Context(), svcCtx).List(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminCategoryCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminCategoryCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminCategoryCreateLogic(r.Context(), svcCtx).Create(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminCategoryUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminCategoryUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewAdminCategoryUpdateLogic(r.Context(), svcCtx).Update(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminCategoryDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminCategoryDeleteReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		err := logic.NewAdminCategoryDeleteLogic(r.Context(), svcCtx).Delete(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, nil)
		}
	}
}
