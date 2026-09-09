package handler

import (
	"context"
	"encoding/json"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/payment-service/internal/points"
	"github.com/askxuan/payment-service/internal/svc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/rest"
	"net/http"
	"strconv"
)

// Report entitlement and points debit commit together on the existing MySQL cluster.
func registerReportPayment(server *rest.Server, s *svc.ServiceContext) {
	auth := &middleware.AuthConfig{Secret: s.Config.Auth.AccessSecret}
	server.AddRoute(rest.Route{Method: "POST", Path: "/api/v1/payments/ai-report", Handler: auth.AuthFunc(func(w http.ResponseWriter, r *http.Request) {
		user := strconv.FormatInt(middleware.UserIDFromCtx(r.Context()), 10)
		if user == "0" {
			common.JsonError(w, common.ErrUnauthorized)
			return
		}
		var req struct {
			ReportID       int64 `json:"reportId"`
			ExpectedPoints int64 `json:"expectedPoints"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil || req.ReportID <= 0 {
			common.JsonError(w, common.ErrParam)
			return
		}
		err := redeemReport(r.Context(), s.DB, user, req.ReportID, req.ExpectedPoints)
		if err != nil {
			if err == points.ErrBalance {
				err = common.NewBizError(common.ErrParam.Code, "积分不足，请先查看积分账户")
			}
			if _, ok := err.(*common.BizError); !ok {
				err = common.ErrSystem
			}
			common.JsonError(w, err)
			return
		}
		common.Ok(w, map[string]bool{"unlocked": true})
	})})
}

func redeemReport(ctx context.Context, db sqlx.SqlConn, user string, reportID, expectedPoints int64) error {
	return db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var report struct {
			No     string `db:"report_no"`
			Price  int64  `db:"points_price"`
			Paid   int64  `db:"points_paid"`
			Status string `db:"status"`
		}
		if e := tx.QueryRowCtx(ctx, &report, `SELECT report_no,points_price,points_paid,status FROM askxuan_ai.ai_report WHERE id=? AND user_id=? FOR UPDATE`, reportID, user); e != nil {
			return common.ErrForbidden
		}
		if report.Paid == 1 {
			return nil
		}
		if report.Status != "ready" || report.Price <= 0 || report.Price != expectedPoints {
			return common.NewBizError(common.ErrParam.Code, "报告状态或价格已变化，请刷新")
		}
		// Prevent two payment methods from charging for the same report.
		var count int64
		if e := tx.QueryRowCtx(ctx, &count, `SELECT COUNT(*) FROM payment WHERE order_type='ai_report' AND order_no=? AND status IN ('pending','success','refunding')`, report.No); e != nil {
			return e
		}
		if count > 0 {
			return common.NewBizError(common.ErrParam.Code, "已有现金支付记录，请刷新报告")
		}
		if e := points.Change(ctx, tx, user, "ai-report:"+report.No, "redeem", report.No, -report.Price); e != nil {
			return e
		}
		_, e := tx.ExecCtx(ctx, `UPDATE askxuan_ai.ai_report SET points_paid=1 WHERE id=?`, reportID)
		return e
	})
}
