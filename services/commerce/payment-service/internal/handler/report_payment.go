package handler

import (
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/payment-service/internal/svc"
	"github.com/zeromicro/go-zero/rest"
	"net/http"
)

var errReportPointsRetired = common.NewBizError(41001, "AI 报告已改为钱包余额支付，请更新页面后重试")

// Keep an explicit tombstone for old clients. This endpoint performs no reads,
// points debits, cash charges or entitlement mutations.
func registerReportPayment(server *rest.Server, s *svc.ServiceContext) {
	auth := &middleware.AuthConfig{Secret: s.Config.Auth.AccessSecret}
	server.AddRoute(rest.Route{Method: http.MethodPost, Path: "/api/v1/payments/ai-report", Handler: auth.AuthFunc(func(w http.ResponseWriter, r *http.Request) {
		common.JsonError(w, errReportPointsRetired)
	})})
}
