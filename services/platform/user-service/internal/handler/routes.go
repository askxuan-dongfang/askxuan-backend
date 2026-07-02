package handler

import (
	"net/http"
	"strconv"

	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/user-service/internal/logic"
	"github.com/askxuan/user-service/internal/svc"
	"github.com/askxuan/user-service/internal/types"

	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// RegisterHandlers 注册 user 服务路由
func RegisterHandlers(server *rest.Server, svcCtx *svc.ServiceContext) {
	server.Use(middleware.CorsFunc)

	// ============ C端用户分组 ============
	server.AddRoutes([]rest.Route{
		{
			Method:  http.MethodPost,
			Path:    "/api/v1/user/register",
			Handler: registerHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/user/profile",
			Handler: profileHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/user/profile",
			Handler: updateProfileHandler(svcCtx),
		},
		// 收货地址
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/user/addresses",
			Handler: addressListHandler(svcCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/api/v1/user/addresses",
			Handler: addressCreateHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/user/addresses/:id",
			Handler: addressUpdateHandler(svcCtx),
		},
		{
			Method:  http.MethodDelete,
			Path:    "/api/v1/user/addresses/:id",
			Handler: addressDeleteHandler(svcCtx),
		},
	})

	// ============ 平台管理台分组 ============
	server.AddRoutes([]rest.Route{
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/users",
			Handler: adminUserListHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/users/:id",
			Handler: adminUserDetailHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/users/:id/status",
			Handler: adminUserStatusHandler(svcCtx),
		},
	})
}

// readUserId 从网关透传的 X-User-Id 头读取 userId
func readUserId(r *http.Request) int64 {
	v := r.Header.Get("X-User-Id")
	if v == "" {
		return 0
	}
	uid, _ := strconv.ParseInt(v, 10, 64)
	return uid
}

func registerHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.RegisterReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewRegisterLogic(r.Context(), svcCtx)
		resp, err := l.Register(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func profileHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req := &types.ProfileReq{UserId: readUserId(r)}
		l := logic.NewProfileLogic(r.Context(), svcCtx)
		resp, err := l.Profile(req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func updateProfileHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.UpdateProfileReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		req.UserId = readUserId(r)
		l := logic.NewUpdateProfileLogic(r.Context(), svcCtx)
		resp, err := l.UpdateProfile(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func addressListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewAddressListLogic(r.Context(), svcCtx)
		resp, err := l.AddressList(readUserId(r))
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func addressCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AddressCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAddressCreateLogic(r.Context(), svcCtx)
		resp, err := l.AddressCreate(&req, readUserId(r))
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func addressUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AddressUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAddressUpdateLogic(r.Context(), svcCtx)
		resp, err := l.AddressUpdate(&req, readUserId(r))
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func addressDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AddressDeleteReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAddressDeleteLogic(r.Context(), svcCtx)
		if err := l.AddressDelete(&req, readUserId(r)); err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, nil)
		}
	}
}

func adminUserListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminUserListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminUserListLogic(r.Context(), svcCtx)
		resp, err := l.AdminUserList(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminUserDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminUserDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminUserDetailLogic(r.Context(), svcCtx)
		resp, err := l.AdminUserDetail(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminUserStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminUserStatusReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminUserStatusLogic(r.Context(), svcCtx)
		resp, err := l.AdminUserStatus(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}
