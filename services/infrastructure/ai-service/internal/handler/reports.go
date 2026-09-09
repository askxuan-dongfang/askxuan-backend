package handler

import (
	"encoding/json"
	"github.com/askxuan/ai-service/internal/logic"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
	"net/http"
	"strconv"
)

func registerReports(server *rest.Server, s *svc.ServiceContext) {
	for _, op := range []struct{ method, path, action string }{{"GET", "/topics", "topics"}, {"POST", "/reports", "create"}, {"GET", "/reports", "list"}, {"GET", "/reports/:id", "detail"}, {"POST", "/reports/:id/retry", "retry"}, {"POST", "/reports/:id/conversation", "conversation"}} {
		action := op.action
		server.AddRoute(rest.Route{Method: op.method, Path: "/api/v1/ai" + op.path, Handler: func(w http.ResponseWriter, r *http.Request) {
			user, err := resolveUserID(r, "")
			if err != nil {
				respond(w, nil, err)
				return
			}
			var data interface{}
			switch action {
			case "topics":
				data, err = logic.ReportProducts(r.Context(), s)
			case "create":
				var req logic.ReportRequest
				if json.NewDecoder(http.MaxBytesReader(w, r.Body, 32768)).Decode(&req) != nil {
					err = common.ErrParam
				} else {
					data, err = logic.ReportCreate(r.Context(), s, user, req)
				}
			case "list":
				page, _ := strconv.Atoi(r.URL.Query().Get("page"))
				data, err = logic.ReportList(r.Context(), s, user, page)
			default:
				var path struct {
					ID int64 `path:"id"`
				}
				if httpx.ParsePath(r, &path) != nil || path.ID <= 0 {
					err = common.ErrParam
				} else if action == "conversation" {
					data, err = logic.ReportConversation(r.Context(), s, user, path.ID)
				} else if action == "retry" {
					data, err = logic.ReportRetry(r.Context(), s, user, path.ID)
				} else {
					data, err = logic.ReportGet(r.Context(), s, user, path.ID)
				}
			}
			respond(w, data, err)
		}})
	}
}
