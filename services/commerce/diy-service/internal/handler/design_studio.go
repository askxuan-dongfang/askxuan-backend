package handler

import (
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/diy-service/internal/logic"
	"github.com/askxuan/diy-service/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
	"net/http"
	"strconv"
	"strings"
)

// Public reads accept an optional verified JWT for owners opening private drafts.
// Gateway GET allowlisting must never make X-User-Id authoritative here.
func optionalDesignAuth(s *svc.ServiceContext, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			(&middleware.AuthConfig{Secret: s.Config.AuthSecret}).AuthFunc(next)(w, r)
			return
		}
		next(w, r)
	}
}
func designStudioAction(s *svc.ServiceContext, action string, admin bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Id       int64  `path:"id"`
			Revision int64  `json:"revision,optional"`
			Status   string `json:"status,optional"`
		}
		if httpx.Parse(r, &req) != nil || req.Id < 1 {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewDesignStudio(r.Context(), s)
		if action == "copy" {
			out, err := l.Copy(req.Id)
			if err != nil {
				common.JsonError(w, err)
			} else {
				common.Ok(w, out)
			}
			return
		}
		out, err := l.ChangeStatus(req.Id, req.Revision, req.Status, admin)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, out)
		}
	}
}
func studioList(s *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		size, _ := strconv.Atoi(r.URL.Query().Get("size"))
		if page < 1 {
			page = 1
		}
		if size < 1 || size > 100 {
			size = 20
		}
		status := r.URL.Query().Get("status")
		if status != "" && status != "private" && status != "public" && status != "pending_review" && status != "approved" && status != "rejected" {
			common.JsonError(w, common.ErrParam)
			return
		}
		list, total, err := s.DiyDesignModel.FindStudioList(r.Context(), "", status, strings.TrimSpace(r.URL.Query().Get("keyword")), page, size)
		if err != nil {
			common.JsonError(w, common.ErrSystem)
			return
		}
		common.Ok(w, map[string]interface{}{"list": list, "total": total, "page": page, "size": size})
	}
}
