package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/askxuan/ai-service/internal/agentops"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func registerAgentOperations(server *rest.Server, s *svc.ServiceContext) {
	for _, op := range []struct{ method, path, action string }{
		{"GET", "", "workspace"}, {"PUT", "", "save"}, {"POST", "/publish", "publish"}, {"POST", "/rollback", "rollback"},
		{"GET", "/debug", "debug-list"}, {"POST", "/debug", "debug-start"}, {"GET", "/debug/:id", "debug-get"}, {"POST", "/debug/:id/resume", "debug-resume"}, {"POST", "/debug/:id/cancel", "debug-cancel"},
		{"GET", "/runs", "runs"}, {"GET", "/runs/:id/tools", "tools"},
		{"GET", "/versions/:id", "version-get"},
	} {
		server.AddRoute(rest.Route{Method: op.method, Path: "/api/v1/ai/admin/agent" + op.path, Handler: agentOperationsHandler(s, op.action)})
	}
}
func agentOperationsHandler(s *svc.ServiceContext, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		actor := r.Header.Get("X-User-Id")
		id, e := strconv.ParseInt(actor, 10, 64)
		super := false
		for _, role := range strings.Split(r.Header.Get("X-User-Roles"), ",") {
			if role == "platform_super" {
				super = true
			}
		}
		if e != nil || id <= 0 || r.Header.Get("X-User-Type") != "admin" || !super {
			common.JsonError(w, common.ErrRoleForbidden)
			return
		}
		m := s.AgentOps
		if m == nil {
			common.JsonError(w, common.NewBizError(50301, "智能体管理尚未初始化"))
			return
		}
		read := func(v any) bool {
			d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 262144))
			d.DisallowUnknownFields()
			if d.Decode(v) != nil {
				common.JsonError(w, common.ErrParam)
				return false
			}
			var extra any
			if d.Decode(&extra) != io.EOF {
				common.JsonError(w, common.ErrParam)
				return false
			}
			return true
		}
		finish := func(v any, err error) {
			if err == nil {
				common.Ok(w, v)
				return
			}
			var input *agentops.RequestError
			switch {
			case errors.As(err, &input):
				common.JsonError(w, common.NewBizError(40001, input.Error()))
			case errors.Is(err, agentops.ErrConflict):
				common.JsonError(w, common.NewBizError(40901, err.Error()))
			case errors.Is(err, agentops.ErrUntested), errors.Is(err, agentops.ErrExpired):
				common.JsonError(w, common.NewBizError(40001, err.Error()))
			default:
				common.JsonError(w, common.NewBizError(50301, "操作暂未完成，请刷新重试；持续失败时检查服务与数据库迁移"))
			}
		}
		var path struct {
			ID string `path:"id"`
		}
		if strings.HasPrefix(action, "debug-") && action != "debug-start" && action != "debug-list" || action == "tools" || action == "version-get" {
			if httpx.ParsePath(r, &path) != nil || path.ID == "" || len(path.ID) > 64 {
				common.JsonError(w, common.ErrParam)
				return
			}
		}
		switch action {
		case "workspace":
			v, err := m.Workspace(r.Context())
			finish(v, err)
		case "version-get":
			id, err := strconv.ParseInt(path.ID, 10, 64)
			if err != nil || id < 1 {
				common.JsonError(w, common.ErrParam)
				return
			}
			v, err := m.Repo.Version(r.Context(), id)
			if err != nil {
				finish(nil, err)
				return
			}
			f, err := agentops.Decode(v.Definition)
			finish(map[string]any{"version": v, "config": f.Config}, err)
		case "save":
			var req struct {
				Revision int64           `json:"revision"`
				Config   agentops.Config `json:"config"`
			}
			if !read(&req) {
				return
			}
			finish(nil, m.Save(r.Context(), req.Revision, req.Config, actor))
		case "publish":
			var req struct {
				Revision int64  `json:"revision"`
				Note     string `json:"note"`
			}
			if !read(&req) {
				return
			}
			v, err := m.Publish(r.Context(), req.Revision, actor, req.Note)
			finish(map[string]any{"version": v}, err)
		case "rollback":
			var req struct {
				Revision int64  `json:"revision"`
				Version  int64  `json:"version"`
				Note     string `json:"note"`
			}
			if !read(&req) {
				return
			}
			finish(nil, m.Rollback(r.Context(), req.Revision, req.Version, actor, req.Note))
		case "debug-start":
			var req agentops.DebugRequest
			if !read(&req) {
				return
			}
			v, err := m.StartDebug(r.Context(), actor, req)
			finish(v, err)
		case "debug-get":
			v, err := m.Debug(r.Context(), path.ID)
			finish(v, err)
		case "debug-list":
			v, err := m.DebugList(r.Context())
			finish(map[string]any{"list": v}, err)
		case "debug-resume":
			var req struct {
				InterruptID string         `json:"interruptId"`
				Inputs      map[string]any `json:"inputs"`
			}
			if !read(&req) {
				return
			}
			finish(nil, m.ResumeDebug(r.Context(), actor, path.ID, req.InterruptID, req.Inputs))
		case "debug-cancel":
			finish(nil, m.CancelDebug(r.Context(), actor, path.ID))
		case "runs":
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			if page < 1 {
				page = 1
			}
			if page > 10000 {
				common.JsonError(w, common.ErrParam)
				return
			}
			status := r.URL.Query().Get("status")
			if status != "" && status != "running" && status != "completed" && status != "failed" {
				common.JsonError(w, common.ErrParam)
				return
			}
			if s.DB == nil {
				finish(nil, agentops.ErrUnavailable)
				return
			}
			repo := agentops.SQLRepository{DB: s.DB}
			rows, more, err := repo.ProductionRuns(r.Context(), page, status)
			finish(map[string]any{"list": rows, "hasMore": more, "page": page}, err)
		case "tools":
			id, err := strconv.ParseInt(path.ID, 10, 64)
			if err != nil || id < 1 {
				common.JsonError(w, common.ErrParam)
				return
			}
			if s.DB == nil {
				finish(nil, agentops.ErrUnavailable)
				return
			}
			repo := agentops.SQLRepository{DB: s.DB}
			v, err := repo.ProductionTools(r.Context(), id)
			finish(map[string]any{"list": v}, err)
		}
	}
}
