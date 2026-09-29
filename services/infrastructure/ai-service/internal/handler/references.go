package handler

import (
	"encoding/json"
	"errors"
	"github.com/askxuan/ai-service/internal/knowledge"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func registerReferences(server *rest.Server, s *svc.ServiceContext) {
	for _, kind := range []string{"memory", "knowledge"} {
		base := "/api/v1/ai/memory"
		if kind == "knowledge" {
			base = "/api/v1/ai/admin/knowledge"
		}
		for _, op := range []struct{ method, path, action string }{{"GET", "", "list"}, {"POST", "", "save"}, {"DELETE", "/:id", "delete"}, {"POST", "/search", "search"}, {"POST", "/import", "import"}} {
			server.AddRoute(rest.Route{Method: op.method, Path: base + op.path, Handler: referenceHandler(s, kind, op.action)})
		}
	}
}
func referenceHandler(s *svc.ServiceContext, kind, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		owner, err := resolveUserID(r, "")
		if err != nil {
			common.JsonError(w, err)
			return
		}
		if kind == "knowledge" {
			super := false
			for _, v := range strings.Split(r.Header.Get("X-User-Roles"), ",") {
				super = super || v == "platform_super"
			}
			id, e := strconv.ParseInt(owner, 10, 64)
			if !super || r.Header.Get("X-User-Type") != "admin" || e != nil || id <= 0 {
				common.JsonError(w, common.ErrRoleForbidden)
				return
			}
			owner = "platform"
		}
		if s.Knowledge == nil {
			common.JsonError(w, common.NewBizError(50301, "知识与记忆服务尚未启用"))
			return
		}
		read := func(v any) bool {
			d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 131072))
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
		finish := func(v any, e error) {
			if e == nil {
				common.Ok(w, v)
			} else if errors.Is(e, knowledge.ErrInput) {
				common.JsonError(w, common.ErrParam)
			} else if errors.Is(e, knowledge.ErrNotFound) {
				common.JsonError(w, common.NewBizError(40901, "资料已变更或不存在，请刷新"))
			} else {
				common.JsonError(w, common.NewBizError(50301, "资料操作失败，请检查数据库迁移、数量限制与嵌入服务"))
			}
		}
		switch action {
		case "import":
			if kind != "knowledge" {
				common.JsonError(w, common.ErrRoleForbidden)
				return
			}
			var req struct {
				Title     string `json:"title"`
				Source    string `json:"source"`
				Text      string `json:"text"`
				Confirmed bool   `json:"confirmed"`
			}
			if !read(&req) {
				return
			}
			if !req.Confirmed {
				common.JsonError(w, common.ErrParam)
				return
			}
			n, e := s.Knowledge.ImportText(r.Context(), req.Title, req.Source, req.Text)
			finish(map[string]any{"count": n}, e)
		case "list":
			v, e := s.Knowledge.List(r.Context(), kind, owner)
			finish(map[string]any{"list": v, "status": s.Knowledge.Status()}, e)
		case "save":
			var req struct {
				Entry     knowledge.Entry `json:"entry"`
				Confirmed bool            `json:"confirmed"`
			}
			if !read(&req) {
				return
			}
			if !req.Confirmed {
				common.JsonError(w, common.NewBizError(40001, "请确认资料内容及来源后保存"))
				return
			}
			req.Entry.Kind = kind
			req.Entry.Owner = owner
			if kind == "memory" {
				req.Entry.Source = "本人确认"
				req.Entry.Locator = "用户维护的跨会话记忆"
			}
			v, e := s.Knowledge.Save(r.Context(), req.Entry)
			finish(v, e)
		case "search":
			var req struct {
				Query string `json:"query"`
			}
			if !read(&req) {
				return
			}
			v, e := s.Knowledge.Search(r.Context(), kind, owner, req.Query)
			finish(v, e)
		case "delete":
			var path struct {
				ID string `path:"id"`
			}
			if httpx.ParsePath(r, &path) != nil {
				common.JsonError(w, common.ErrParam)
				return
			}
			finish(nil, s.Knowledge.Delete(r.Context(), kind, owner, path.ID))
		}
	}
}
