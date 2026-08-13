package handler

import (
	"net/http"

	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/temple-service/internal/logic"
	"github.com/askxuan/temple-service/internal/svc"
	"github.com/askxuan/temple-service/internal/types"

	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// RegisterHandlers 注册 temple 服务路由
func RegisterHandlers(server *rest.Server, svcCtx *svc.ServiceContext) {
	server.Use(middleware.CorsFunc)

	// ============ C端分组（公开） ============
	server.AddRoutes([]rest.Route{
		{Method: http.MethodGet, Path: "/api/v1/beliefs", Handler: beliefListHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/beliefs/:code", Handler: beliefDetailHandler(svcCtx)},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/temples",
			Handler: listHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/temples/:id",
			Handler: detailHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/temples/:id/services",
			Handler: serviceListHandler(svcCtx),
		},
	})

	// ============ 寺院管理台分组（需JWT，由网关鉴权） ============
	// adminContextMiddleware 将网关注入的 X-Temple-Id / X-User-Id 请求头解析到 context
	adminRoutes := []rest.Route{
		// 寺院信息
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/temples/info",
			Handler: adminTempleInfoHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/temples/info",
			Handler: adminTempleUpdateHandler(svcCtx),
		},
		// 图片管理
		{
			Method:  http.MethodPost,
			Path:    "/api/v1/admin/temples/images",
			Handler: adminImageCreateHandler(svcCtx),
		},
		{
			Method:  http.MethodDelete,
			Path:    "/api/v1/admin/temples/images/:id",
			Handler: adminImageDeleteHandler(svcCtx),
		},
		// 服务管理
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/temples/services",
			Handler: adminServiceListHandler(svcCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/api/v1/admin/temples/services",
			Handler: adminServiceCreateHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/temples/services/:id",
			Handler: adminServiceUpdateHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/temples/services/:id/status",
			Handler: adminServiceStatusHandler(svcCtx),
		},
		// 加持任务（修复 Gap-3）
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/temples/blessing-tasks",
			Handler: adminBlessingTaskListHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/temples/blessing-tasks/:id",
			Handler: adminBlessingTaskDetailHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/temples/blessing-tasks/:id/assign",
			Handler: adminBlessingAssignHandler(svcCtx),
		},
		// 入驻申请
		{
			Method:  http.MethodPost,
			Path:    "/api/v1/admin/temples/apply",
			Handler: adminTempleApplyHandler(svcCtx),
		},
		// 寺院报表
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/temples/reports",
			Handler: adminTempleReportsHandler(svcCtx),
		},
	}
	server.AddRoutes(rest.WithMiddleware(adminContextMiddleware, adminRoutes...))

	// ============ 平台管理台分组（需JWT + 平台超管） ============
	platformRoutes := []rest.Route{
		{Method: http.MethodGet, Path: "/api/v1/admin/platform/beliefs", Handler: platformBeliefListHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/platform/beliefs", Handler: platformBeliefCreateHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/admin/platform/beliefs/:code", Handler: platformBeliefUpdateHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/admin/platform/beliefs/:code/status", Handler: platformBeliefStatusHandler(svcCtx)},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/platform/temples",
			Handler: platformTempleListHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/platform/temples/audits",
			Handler: platformAuditListHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/platform/temples/audits/:id/first-pass",
			Handler: platformAuditFirstPassHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/platform/temples/audits/:id/final-pass",
			Handler: platformAuditFinalPassHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/platform/temples/audits/:id/reject",
			Handler: platformAuditRejectHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/platform/temples/:id",
			Handler: platformTempleDetailHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/platform/temples/:id/status",
			Handler: platformTempleStatusHandler(svcCtx),
		},
	}
	server.AddRoutes(rest.WithMiddleware(adminContextMiddleware, platformRoutes...))
}

func beliefListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := logic.ListBeliefs(r.Context(), svcCtx, false)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

func platformBeliefListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp, err := logic.ListBeliefs(r.Context(), svcCtx, true)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

func platformBeliefCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.BeliefCreateReq
		if httpx.Parse(r, &req) != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.CreateBelief(r.Context(), svcCtx, &req)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

func platformBeliefStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.BeliefStatusReq
		if httpx.Parse(r, &req) != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		if err := logic.UpdateBeliefStatus(r.Context(), svcCtx, &req); err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, req)
	}
}

func beliefDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.BeliefReq
		if httpx.Parse(r, &req) != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.GetBelief(r.Context(), svcCtx, req.Code)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

func platformBeliefUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.BeliefUpdateReq
		if httpx.Parse(r, &req) != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.UpdateBelief(r.Context(), svcCtx, &req)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

// adminContextMiddleware 将网关注入的 X-Temple-Id / X-User-Id 请求头解析到 context
// 网关（gateway-service）已校验 JWT 并注入这些头部，此处仅做透传
func adminContextMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		templeId := svc.ParseHeaderToInt64(r.Header.Get(svc.HeaderTempleID))
		userId := svc.ParseHeaderToInt64(r.Header.Get(svc.HeaderUserID))
		if templeId > 0 {
			ctx = svc.WithTempleID(ctx, templeId)
		}
		if userId > 0 {
			ctx = svc.WithUserID(ctx, userId)
		}
		next(w, r.WithContext(ctx))
	}
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

func serviceListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.TempleServiceListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewServiceListLogic(r.Context(), svcCtx)
		resp, err := l.ServiceList(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

// ============ 寺院管理台 Handler ============

func adminTempleInfoHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminTempleInfoLogic(r.Context(), svcCtx)
		resp, err := l.AdminTempleInfo(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminTempleUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.TempleUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminTempleUpdateLogic(r.Context(), svcCtx)
		resp, err := l.AdminTempleUpdate(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminImageCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.TempleImageCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminImageCreateLogic(r.Context(), svcCtx)
		resp, err := l.AdminImageCreate(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminImageDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.TempleImageDeleteReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminImageDeleteLogic(r.Context(), svcCtx)
		if err := l.AdminImageDelete(&req); err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, nil)
		}
	}
}

func adminServiceListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := logic.NewAdminServiceListLogic(r.Context(), svcCtx)
		resp, err := l.AdminServiceList()
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminServiceCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.TempleServiceCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminServiceCreateLogic(r.Context(), svcCtx)
		resp, err := l.AdminServiceCreate(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminServiceUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.TempleServiceUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminServiceUpdateLogic(r.Context(), svcCtx)
		resp, err := l.AdminServiceUpdate(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminServiceStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.TempleServiceStatusReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminServiceStatusLogic(r.Context(), svcCtx)
		resp, err := l.AdminServiceStatus(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminBlessingTaskListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.BlessingTaskListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminBlessingTaskListLogic(r.Context(), svcCtx)
		resp, err := l.AdminBlessingTaskList(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminBlessingTaskDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.BlessingTaskDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminBlessingTaskDetailLogic(r.Context(), svcCtx)
		resp, err := l.AdminBlessingTaskDetail(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminBlessingAssignHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.BlessingAssignReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminBlessingAssignLogic(r.Context(), svcCtx)
		resp, err := l.AdminBlessingAssign(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminTempleApplyHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.TempleApplyReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminTempleApplyLogic(r.Context(), svcCtx)
		resp, err := l.AdminTempleApply(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func adminTempleReportsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.TempleReportReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAdminTempleReportsLogic(r.Context(), svcCtx)
		resp, err := l.AdminTempleReports(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

// ============ 平台管理台 Handler ============

func platformTempleListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewPlatformTempleListLogic(r.Context(), svcCtx)
		resp, err := l.PlatformTempleList(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func platformTempleDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.PlatformTempleDetail(r.Context(), svcCtx, &req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func platformAuditListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.TempleAuditListReq
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

func platformAuditFirstPassHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.TempleAuditActionReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewPlatformAuditFirstPassLogic(r.Context(), svcCtx)
		resp, err := l.PlatformAuditFirstPass(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func platformAuditFinalPassHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.TempleAuditActionReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewPlatformAuditFinalPassLogic(r.Context(), svcCtx)
		resp, err := l.PlatformAuditFinalPass(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func platformAuditRejectHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.TempleAuditActionReq
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

func platformTempleStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.TemplePlatformStatusReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewPlatformTempleStatusLogic(r.Context(), svcCtx)
		resp, err := l.PlatformTempleStatus(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}
