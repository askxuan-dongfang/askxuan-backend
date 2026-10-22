package handler

import (
	"encoding/json"
	"github.com/askxuan/booking-service/internal/logic"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"net/http"
	"strconv"
)

func registerTempleExceptions(server *rest.Server, s *svc.ServiceContext, auth *middleware.AuthConfig) {
	for _, platform := range []bool{false, true} {
		platform := platform
		prefix := "/api/v1/admin/temples/exceptions"
		if platform {
			prefix = "/api/v1/admin/platform/temple-exceptions"
		}
		server.AddRoute(rest.Route{Method: "GET", Path: prefix, Handler: auth.AuthFunc(func(w http.ResponseWriter, r *http.Request) {
			page := 1
			var e error
			if p := r.URL.Query().Get("page"); p != "" {
				page, e = strconv.Atoi(p)
			}
			if e != nil {
				common.JsonError(w, common.ErrParam)
				return
			}
			out, e := logic.TempleExceptions(r.Context(), s, r.URL.Query().Get("templeCode"), platform, page)
			replyFulfillment(w, out, e)
		})})
		server.AddRoute(rest.Route{Method: "GET", Path: prefix + "/:id", Handler: auth.AuthFunc(func(w http.ResponseWriter, r *http.Request) {
			rows, e := logic.ExceptionNotes(r.Context(), s, pathvar.Vars(r)["id"], platform)
			replyFulfillment(w, map[string]any{"list": rows}, e)
		})})
	}
	server.AddRoute(rest.Route{Method: "POST", Path: "/api/v1/admin/temples/exceptions/:id/notes", Handler: auth.AuthFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			RequestID string `json:"requestId"`
			Action    string `json:"action"`
			Content   string `json:"content"`
		}
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 8000)).Decode(&req) != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		e := logic.AddExceptionNote(r.Context(), s, pathvar.Vars(r)["id"], req.RequestID, req.Action, req.Content)
		replyFulfillment(w, map[string]bool{"saved": e == nil}, e)
	})})
}
