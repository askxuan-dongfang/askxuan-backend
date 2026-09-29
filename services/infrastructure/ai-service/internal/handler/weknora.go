package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/ai-service/internal/weknora"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func registerWeKnora(server *rest.Server, s *svc.ServiceContext) {
	for _, v := range []struct{ method, path, action string }{
		{"GET", "", "list"}, {"POST", "", "create"}, {"PUT", "/:kb", "update"}, {"DELETE", "/:kb", "delete"},
		{"GET", "/:kb/documents", "documents"}, {"POST", "/:kb/documents", "manual"}, {"POST", "/:kb/upload", "upload"},
		{"GET", "/:kb/documents/:doc/chunks", "chunks"}, {"PUT", "/:kb/documents/:doc", "policy"}, {"DELETE", "/:kb/documents/:doc", "delete_doc"}, {"POST", "/:kb/documents/:doc/reparse", "reparse"},
		{"POST", "/:kb/search", "search"},
	} {
		server.AddRoute(rest.Route{Method: v.method, Path: "/api/v1/ai/admin/knowledge-bases" + v.path, Handler: weknoraHandler(s, v.action)}, rest.WithMaxBytes(weknora.MaxFileBytes+131072))
	}
}
func weknoraHandler(s *svc.ServiceContext, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		actor, e := resolveUserID(r, "")
		n, _ := strconv.ParseInt(actor, 10, 64)
		super := false
		for _, role := range strings.Split(r.Header.Get("X-User-Roles"), ",") {
			if role == "platform_super" {
				super = true
			}
		}
		if e != nil || n <= 0 || r.Header.Get("X-User-Type") != "admin" || !super {
			common.JsonError(w, common.ErrRoleForbidden)
			return
		}
		if s.WeKnora == nil {
			common.JsonError(w, common.NewBizError(50301, "WeKnora 尚未配置"))
			return
		}
		finish := func(v any, e error) {
			if e == nil {
				common.Ok(w, v)
				return
			}
			if errors.Is(e, weknora.ErrInput) {
				common.JsonError(w, common.NewBizError(40001, e.Error()))
			} else if errors.Is(e, weknora.ErrConflict) {
				common.JsonError(w, common.NewBizError(40901, e.Error()))
			} else {
				common.JsonError(w, common.NewBizError(50301, "知识操作失败，请检查引擎、索引及数据库状态"))
			}
		}
		var path struct {
			KB  string `path:"kb,optional"`
			Doc string `path:"doc,optional"`
		}
		_ = httpx.ParsePath(r, &path)
		read := func(v any) bool {
			d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 250000))
			d.DisallowUnknownFields()
			var extra any
			if d.Decode(v) != nil || d.Decode(&extra) != io.EOF {
				common.JsonError(w, common.ErrParam)
				return false
			}
			return true
		}
		page := 1
		if p := r.URL.Query().Get("page"); p != "" {
			page, _ = strconv.Atoi(p)
		}
		ctx := r.Context()
		wk := s.WeKnora
		switch action {
		case "list":
			v, e := wk.Bases(ctx)
			finish(map[string]any{"list": v, "engine": "WeKnora", "version": "v0.8.2"}, e)
		case "create":
			var v struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			}
			if !read(&v) {
				return
			}
			out, e := wk.CreateBase(ctx, v.Name, v.Description, actor)
			finish(out, e)
		case "update":
			var v weknora.Base
			if !read(&v) {
				return
			}
			v.ID = path.KB
			finish(nil, wk.UpdateBase(ctx, v, actor))
		case "delete":
			finish(nil, wk.DeleteBase(ctx, path.KB, actor))
		case "documents":
			v, e := wk.Documents(ctx, path.KB, page)
			finish(v, e)
		case "manual":
			var v struct {
				Title   string `json:"title"`
				Content string `json:"content"`
				Source  string `json:"source"`
			}
			if !read(&v) {
				return
			}
			out, e := wk.Manual(ctx, path.KB, v.Title, v.Content, v.Source, actor)
			finish(out, e)
		case "upload":
			r.Body = http.MaxBytesReader(w, r.Body, weknora.MaxFileBytes+131072)
			if r.ParseMultipartForm(1024*1024) != nil {
				common.JsonError(w, common.ErrParam)
				return
			}
			defer r.MultipartForm.RemoveAll()
			f, h, e := r.FormFile("file")
			if e != nil {
				common.JsonError(w, common.ErrParam)
				return
			}
			defer f.Close()
			v, e := wk.Upload(ctx, path.KB, h.Filename, r.FormValue("source"), actor, f)
			finish(v, e)
		case "chunks":
			v, e := wk.Chunks(ctx, path.KB, path.Doc, page)
			finish(v, e)
		case "policy":
			var p weknora.Policy
			if !read(&p) {
				return
			}
			p.ID = path.Doc
			p.BaseID = path.KB
			finish(nil, wk.Policy(ctx, p, actor))
		case "delete_doc":
			finish(nil, wk.MutateDocument(ctx, path.KB, path.Doc, "delete", actor))
		case "reparse":
			finish(nil, wk.MutateDocument(ctx, path.KB, path.Doc, "reparse", actor))
		case "search":
			var v struct {
				Query string `json:"query"`
			}
			if !read(&v) {
				return
			}
			out, e := wk.Search(ctx, v.Query, []string{path.KB})
			finish(out, e)
		}
	}
}
