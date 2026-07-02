package handler

import (
	"net/http"

	"github.com/askxuan/auth-service/internal/logic"
	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/auth-service/internal/types"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"

	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// RegisterHandlers 注册 auth 服务路由
func RegisterHandlers(server *rest.Server, svcCtx *svc.ServiceContext) {
	// CORS 中间件
	server.Use(middleware.CorsFunc)

	// ============ C端/通用认证（公开） ============
	server.AddRoutes([]rest.Route{
		{
			Method:  http.MethodPost,
			Path:    "/api/v1/auth/login",
			Handler: loginHandler(svcCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/api/v1/auth/refresh",
			Handler: refreshHandler(svcCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/api/v1/auth/logout",
			Handler: logoutHandler(svcCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/api/v1/auth/admin/login",
			Handler: adminLoginHandler(svcCtx),
		},
	})

	// ============ 管理台分组（需JWT，由网关鉴权） ============
	server.AddRoutes([]rest.Route{
		// 账号管理
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/auth/accounts",
			Handler: adminAccountListHandler(svcCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/api/v1/admin/auth/accounts",
			Handler: adminAccountCreateHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/auth/accounts/:id",
			Handler: adminAccountUpdateHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/auth/accounts/:id/status",
			Handler: adminAccountStatusHandler(svcCtx),
		},
		// 角色管理
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/auth/roles",
			Handler: adminRoleListHandler(svcCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/api/v1/admin/auth/roles",
			Handler: adminRoleCreateHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/auth/roles/:id",
			Handler: adminRoleUpdateHandler(svcCtx),
		},
		// 权限管理
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/auth/permissions",
			Handler: adminPermissionListHandler(svcCtx),
		},
	})
}

func loginHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.LoginReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewLoginLogic(r.Context(), svcCtx)
		resp, err := l.Login(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func refreshHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.RefreshReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewRefreshLogic(r.Context(), svcCtx)
		resp, err := l.Refresh(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func logoutHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.LogoutReq
		_ = httpx.Parse(r, &req)
		l := logic.NewLogoutLogic(r.Context(), svcCtx)
		resp, err := l.Logout(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminLoginHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminLoginReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminLoginLogic(r.Context(), svcCtx)
		resp, err := l.AdminLogin(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminAccountListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminAccountListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminAccountListLogic(r.Context(), svcCtx)
		resp, err := l.AdminAccountList(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminAccountCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminAccountCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminAccountCreateLogic(r.Context(), svcCtx)
		resp, err := l.AdminAccountCreate(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminAccountUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminAccountUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminAccountUpdateLogic(r.Context(), svcCtx)
		resp, err := l.AdminAccountUpdate(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminAccountStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminAccountStatusReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminAccountStatusLogic(r.Context(), svcCtx)
		resp, err := l.AdminAccountStatus(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminRoleListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewAdminRoleListLogic(r.Context(), svcCtx)
		resp, err := l.AdminRoleList()
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminRoleCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.RoleCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminRoleCreateLogic(r.Context(), svcCtx)
		resp, err := l.AdminRoleCreate(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminRoleUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.RoleUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminRoleUpdateLogic(r.Context(), svcCtx)
		resp, err := l.AdminRoleUpdate(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminPermissionListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewAdminPermissionListLogic(r.Context(), svcCtx)
		resp, err := l.AdminPermissionList()
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}
