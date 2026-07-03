package handler

import (
	"net/http"

	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/finance-service/internal/logic"
	"github.com/askxuan/finance-service/internal/svc"
	"github.com/askxuan/finance-service/internal/types"

	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// RegisterHandlers 注册 finance 服务路由
func RegisterHandlers(server *rest.Server, svcCtx *svc.ServiceContext) {
	server.Use(middleware.CorsFunc)

	// JWT 鉴权配置（法师工作台接口需要登录）
	authCfg := &middleware.AuthConfig{Secret: svcCtx.Config.AuthSecret}

	server.AddRoutes([]rest.Route{
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/finance/overview",
			Handler: overviewHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/finance/settlements",
			Handler: settlementListHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/finance/settlements/:id",
			Handler: settlementDetailHandler(svcCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/api/v1/admin/finance/settlements/confirm/:id",
			Handler: settlementConfirmHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/finance/withdrawals",
			Handler: withdrawalListHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/finance/withdrawals/:id/audit",
			Handler: withdrawalAuditHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/finance/withdrawals/:id/process",
			Handler: withdrawalProcessHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/finance/commission-config",
			Handler: commissionConfigListHandler(svcCtx),
		},
		{
			Method:  http.MethodPut,
			Path:    "/api/v1/admin/finance/commission-config/:id",
			Handler: commissionConfigUpdateHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/finance/reports",
			Handler: reportsHandler(svcCtx),
		},
		{
			Method:  http.MethodGet,
			Path:    "/api/v1/admin/finance/shop/reports",
			Handler: shopReportsHandler(svcCtx),
		},
	})

	// ============ 法师工作台分组（需JWT，masterId 从 JWT 获取） ============
	server.AddRoutes(rest.WithMiddleware(authCfg.AuthFunc, []rest.Route{
		{
			Method:  http.MethodPost,
			Path:    "/api/v1/admin/finance/withdrawals/apply",
			Handler: withdrawalApplyHandler(svcCtx),
		},
	}...))
}

func overviewHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.OverviewReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewOverviewLogic(r.Context(), svcCtx)
		resp, err := l.Overview(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func settlementListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SettlementListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewSettlementListLogic(r.Context(), svcCtx)
		resp, err := l.SettlementList(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func settlementDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SettlementDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewSettlementDetailLogic(r.Context(), svcCtx)
		resp, err := l.SettlementDetail(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func settlementConfirmHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SettlementConfirmReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewSettlementConfirmLogic(r.Context(), svcCtx)
		resp, err := l.SettlementConfirm(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func withdrawalListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.WithdrawalListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewWithdrawalListLogic(r.Context(), svcCtx)
		resp, err := l.WithdrawalList(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func withdrawalAuditHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.WithdrawalAuditReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewWithdrawalAuditLogic(r.Context(), svcCtx)
		resp, err := l.WithdrawalAudit(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func withdrawalProcessHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.WithdrawalProcessReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewWithdrawalProcessLogic(r.Context(), svcCtx)
		resp, err := l.WithdrawalProcess(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func withdrawalApplyHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.WithdrawalApplyReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewWithdrawalApplyLogic(r.Context(), svcCtx)
		resp, err := l.WithdrawalApply(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func commissionConfigListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CommissionConfigListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewCommissionConfigListLogic(r.Context(), svcCtx)
		resp, err := l.CommissionConfigList(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func commissionConfigUpdateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CommissionConfigUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewCommissionConfigUpdateLogic(r.Context(), svcCtx)
		resp, err := l.CommissionConfigUpdate(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func reportsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ReportReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewReportsLogic(r.Context(), svcCtx)
		resp, err := l.Reports(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

func shopReportsHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ShopReportReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewShopReportsLogic(r.Context(), svcCtx)
		resp, err := l.ShopReports(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}
