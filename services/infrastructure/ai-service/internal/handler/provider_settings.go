package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/askxuan/ai-service/internal/settings"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
)

func registerProviderSettings(server *rest.Server, s *svc.ServiceContext) {
	for _, op := range []struct{ method, path, action string }{{"GET", "/provider", "get"}, {"PUT", "/provider", "save"}, {"POST", "/provider/test", "test"}} {
		server.AddRoute(rest.Route{Method: op.method, Path: "/api/v1/ai/admin" + op.path, Handler: providerSettingsHandler(s, op.action)})
	}
}
func providerSettingsHandler(s *svc.ServiceContext, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		actor := r.Header.Get("X-User-Id")
		id, err := strconv.ParseInt(actor, 10, 64)
		allowed := false
		for _, role := range strings.Split(r.Header.Get("X-User-Roles"), ",") {
			if role == "platform_super" {
				allowed = true
			}
		}
		// These headers are injected by the authenticated gateway; service ports are private.
		if err != nil || id <= 0 || r.Header.Get("X-User-Type") != "admin" || !allowed {
			common.JsonError(w, common.ErrRoleForbidden)
			return
		}
		if s.Settings == nil {
			common.JsonError(w, common.NewBizError(50301, "配置服务暂不可用"))
			return
		}
		if action == "get" {
			respond(w, s.Settings.Public(), nil)
			return
		}
		var req settings.Update
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&req) != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		var extra any
		if decoder.Decode(&extra) != io.EOF {
			common.JsonError(w, common.ErrParam)
			return
		}
		if action == "test" {
			list, e := s.Settings.Test(r.Context(), req)
			if e != nil {
				common.JsonError(w, common.NewBizError(40001, e.Error()))
				return
			}
			respond(w, list, nil)
			return
		}
		result, e := s.Settings.Save(r.Context(), req, actor)
		if e != nil {
			common.JsonError(w, common.NewBizError(40001, e.Error()))
			return
		}
		respond(w, result, nil)
	}
}

// go-zero's default logger dumps request bodies on HTTP 5xx. Provider credentials
// must never enter logs, including failed requests; record metadata only instead.
func SafeRequestLog(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		defer func() {
			logx.Infof("AI HTTP method=%s path=%s duration_ms=%d", r.Method, r.URL.Path, time.Since(started).Milliseconds())
		}()
		next(w, r)
	}
}
