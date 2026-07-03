package handler

import (
	"net/http"

	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/master-service/internal/logic"
	"github.com/askxuan/master-service/internal/svc"
	"github.com/askxuan/master-service/internal/types"

	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// RegisterHandlers 注册 master 服务路由
func RegisterHandlers(server *rest.Server, svcCtx *svc.ServiceContext) {
	server.Use(middleware.CorsFunc)

	// JWT 鉴权配置（管理台/工作台接口需要登录）
	authCfg := &middleware.AuthConfig{Secret: svcCtx.Config.AuthSecret}

	// ============ C端分组（公开） ============
	server.AddRoutes([]rest.Route{
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/masters",
			Handler: listHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/masters/:id",
			Handler: detailHandler(svcCtx),
		},
	})

	// ============ 寺院管理台分组（需JWT） ============
	server.AddRoutes(rest.WithMiddleware(authCfg.AuthFunc, []rest.Route{
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/temples/masters",
			Handler: adminMasterListHandler(svcCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/api/v1/admin/temples/masters",
			Handler: adminMasterCreateHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/temples/masters/:id",
			Handler: adminMasterUpdateHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/temples/masters/:id/status",
			Handler: adminMasterStatusHandler(svcCtx),
		},
	}...))

	// ============ 法师工作台分组（需JWT + 法师，修复 Gap-15） ============
	server.AddRoutes(rest.WithMiddleware(authCfg.AuthFunc, []rest.Route{
		// 加持任务
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/masters/blessing-tasks",
			Handler: workspaceBlessingTaskListHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/masters/blessing-tasks/:id",
			Handler: workspaceBlessingTaskDetailHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/masters/blessing-tasks/:id/accept",
			Handler: workspaceBlessingAcceptHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/masters/blessing-tasks/:id/start",
			Handler: workspaceBlessingStartHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/masters/blessing-tasks/:id/complete",
			Handler: workspaceBlessingCompleteHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/masters/blessing-tasks/:id/reject",
			Handler: workspaceBlessingRejectHandler(svcCtx),
		},
		// 日程管理
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/masters/schedules",
			Handler: workspaceScheduleListHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/masters/schedules",
			Handler: workspaceScheduleUpdateHandler(svcCtx),
		},
		// 收益管理
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/masters/earnings/summary",
			Handler: workspaceEarningsSummaryHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/masters/earnings/details",
			Handler: workspaceEarningsDetailsHandler(svcCtx),
		},
		// 个人资料
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/masters/profile",
			Handler: workspaceProfileGetHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/masters/profile",
			Handler: workspaceProfileUpdateHandler(svcCtx),
		},
	}...))

	// ============ 平台管理台分组（需JWT + 平台超管） ============
	server.AddRoutes(rest.WithMiddleware(authCfg.AuthFunc, []rest.Route{
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/platform/masters/audits",
			Handler: platformAuditListHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/platform/masters/audits/:id/pass",
			Handler: platformAuditPassHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/platform/masters/audits/:id/reject",
			Handler: platformAuditRejectHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/platform/masters/:id/status",
			Handler: platformMasterStatusHandler(svcCtx),
		},
	}...))
}

// ============ C端 Handler ============

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

// ============ 寺院管理台 Handler ============

func adminMasterListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminMasterListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminMasterListLogic(r.Context(), svcCtx)
		resp, err := l.AdminMasterList(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminMasterCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminMasterCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminMasterCreateLogic(r.Context(), svcCtx)
		resp, err := l.AdminMasterCreate(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminMasterUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminMasterUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminMasterUpdateLogic(r.Context(), svcCtx)
		resp, err := l.AdminMasterUpdate(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminMasterStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AdminMasterStatusReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminMasterStatusLogic(r.Context(), svcCtx)
		resp, err := l.AdminMasterStatus(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

// ============ 法师工作台 Handler（修复 Gap-15） ============

func workspaceBlessingTaskListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.BlessingTaskListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewWorkspaceBlessingTaskListLogic(r.Context(), svcCtx)
		resp, err := l.WorkspaceBlessingTaskList(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func workspaceBlessingTaskDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.BlessingTaskDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewWorkspaceBlessingTaskDetailLogic(r.Context(), svcCtx)
		resp, err := l.WorkspaceBlessingTaskDetail(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func workspaceBlessingAcceptHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.BlessingTaskActionReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewWorkspaceBlessingAcceptLogic(r.Context(), svcCtx)
		resp, err := l.WorkspaceBlessingAccept(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func workspaceBlessingStartHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.BlessingTaskActionReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewWorkspaceBlessingStartLogic(r.Context(), svcCtx)
		resp, err := l.WorkspaceBlessingStart(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func workspaceBlessingCompleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.BlessingTaskCompleteReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewWorkspaceBlessingCompleteLogic(r.Context(), svcCtx)
		resp, err := l.WorkspaceBlessingComplete(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func workspaceBlessingRejectHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.BlessingTaskActionReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewWorkspaceBlessingRejectLogic(r.Context(), svcCtx)
		resp, err := l.WorkspaceBlessingReject(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func workspaceScheduleListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ScheduleListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewWorkspaceScheduleListLogic(r.Context(), svcCtx)
		resp, err := l.WorkspaceScheduleList(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func workspaceScheduleUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ScheduleUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewWorkspaceScheduleUpdateLogic(r.Context(), svcCtx)
		resp, err := l.WorkspaceScheduleUpdate(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

// ============ 平台管理台 Handler ============

func platformAuditListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MasterAuditListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewPlatformAuditListLogic(r.Context(), svcCtx)
		resp, err := l.PlatformAuditList(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func platformAuditPassHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MasterAuditActionReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewPlatformAuditPassLogic(r.Context(), svcCtx)
		resp, err := l.PlatformAuditPass(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func platformAuditRejectHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MasterAuditActionReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewPlatformAuditRejectLogic(r.Context(), svcCtx)
		resp, err := l.PlatformAuditReject(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func platformMasterStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MasterPlatformStatusReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewPlatformMasterStatusLogic(r.Context(), svcCtx)
		resp, err := l.PlatformMasterStatus(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

// ============ 法师工作台 - 收益 Handler ============

func workspaceEarningsSummaryHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.EarningsSummaryReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewWorkspaceEarningsSummaryLogic(r.Context(), svcCtx)
		resp, err := l.WorkspaceEarningsSummary(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func workspaceEarningsDetailsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.EarningsDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewWorkspaceEarningsDetailsLogic(r.Context(), svcCtx)
		resp, err := l.WorkspaceEarningsDetails(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

// ============ 法师工作台 - 个人资料 Handler ============

func workspaceProfileGetHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MasterProfileReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewWorkspaceProfileGetLogic(r.Context(), svcCtx)
		resp, err := l.WorkspaceProfileGet(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func workspaceProfileUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MasterProfileUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewWorkspaceProfileUpdateLogic(r.Context(), svcCtx)
		resp, err := l.WorkspaceProfileUpdate(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}
