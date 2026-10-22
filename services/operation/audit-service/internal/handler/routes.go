package handler

import (
	"context"
	"net/http"

	"github.com/askxuan/audit-service/internal/logic"
	"github.com/askxuan/audit-service/internal/svc"
	"github.com/askxuan/audit-service/internal/types"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"

	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// RegisterHandlers 注册 audit 服务路由
func RegisterHandlers(server *rest.Server, svcCtx *svc.ServiceContext) {
	server.Use(middleware.CorsFunc)
	registerAdminOperations(server, svcCtx)

	server.AddRoutes([]rest.Route{
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/audit/queue",
			Handler: auditQueueListHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/audit/queue/:id",
			Handler: auditQueueDetailHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/audit/queue/:id/approve",
			Handler: auditApproveHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/audit/queue/:id/reject",
			Handler: auditRejectHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/audit/reports",
			Handler: reportListHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/audit/reports/:id/handle",
			Handler: reportHandleHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/audit/sensitive-words",
			Handler: sensitiveWordListHandler(svcCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/api/v1/admin/audit/sensitive-words",
			Handler: sensitiveWordCreateHandler(svcCtx),
		},
		{
			Method:  http.MethodDelete,
			Path:    "/api/v1/admin/audit/sensitive-words/:id",
			Handler: sensitiveWordDeleteHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/audit/statistics",
			Handler: statisticsHandler(svcCtx),
		},
	})
}

func auditQueueListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AuditQueueListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAuditQueueListLogic(r.Context(), svcCtx)
		resp, err := l.AuditQueueList(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func auditQueueDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AuditQueueDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewAuditQueueDetailLogic(r.Context(), svcCtx)
		resp, err := l.AuditQueueDetail(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func auditApproveHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return auditApproveWithAction(svcCtx, func(ctx context.Context, req *types.AuditApproveReq) (*types.AuditApproveResp, error) {
		return logic.NewAuditApproveLogic(ctx, svcCtx).AuditApprove(req)
	})
}

func auditApproveWithAction(svcCtx *svc.ServiceContext, action func(context.Context, *types.AuditApproveReq) (*types.AuditApproveResp, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, err := humanAuditActor(r, svcCtx.Config.Auth.AccessSecret, true)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		var req types.AuditApproveReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		// Retain the old JSON field for clients, but only the signed human may act.
		req.AuditorId = actor
		resp, err := action(r.Context(), &req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func auditRejectHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return auditRejectWithAction(svcCtx, func(ctx context.Context, req *types.AuditRejectReq) (*types.AuditRejectResp, error) {
		return logic.NewAuditRejectLogic(ctx, svcCtx).AuditReject(req)
	})
}

func auditRejectWithAction(svcCtx *svc.ServiceContext, action func(context.Context, *types.AuditRejectReq) (*types.AuditRejectResp, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, err := humanAuditActor(r, svcCtx.Config.Auth.AccessSecret, true)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		var req types.AuditRejectReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		req.AuditorId = actor
		resp, err := action(r.Context(), &req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func reportListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ReportListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewReportListLogic(r.Context(), svcCtx)
		resp, err := l.ReportList(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func reportHandleHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return reportHandleWithAction(svcCtx, func(ctx context.Context, req *types.ReportHandleReq) (*types.ReportHandleResp, error) {
		return logic.NewReportHandleLogic(ctx, svcCtx).ReportHandle(req)
	})
}

func reportHandleWithAction(svcCtx *svc.ServiceContext, action func(context.Context, *types.ReportHandleReq) (*types.ReportHandleResp, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, err := humanAuditActor(r, svcCtx.Config.Auth.AccessSecret, false)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		var req types.ReportHandleReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		req.HandlerId = actor
		resp, err := action(r.Context(), &req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func sensitiveWordListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SensitiveWordListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewSensitiveWordListLogic(r.Context(), svcCtx)
		resp, err := l.SensitiveWordList(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func sensitiveWordCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SensitiveWordCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewSensitiveWordCreateLogic(r.Context(), svcCtx)
		resp, err := l.SensitiveWordCreate(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func sensitiveWordDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SensitiveWordDeleteReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewSensitiveWordDeleteLogic(r.Context(), svcCtx)
		if err := l.SensitiveWordDelete(&req); err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, nil)
		}
	}
}

func statisticsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.StatisticsReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewStatisticsLogic(r.Context(), svcCtx)
		resp, err := l.Statistics(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}
