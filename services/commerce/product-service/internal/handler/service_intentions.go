package handler

import (
	"github.com/askxuan/common"
	"github.com/askxuan/product-service/internal/model"
	"github.com/askxuan/product-service/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
	"net/http"
	"strconv"
	"strings"
)

// Gateway strips external identity headers and verifies admin JWTs.
func serviceIntentions(s *svc.ServiceContext, write bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, _ := strconv.ParseInt(r.Header.Get("X-User-Id"), 10, 64)
		if uid <= 0 || r.Header.Get("X-User-Type") != "admin" || !strings.Contains(","+r.Header.Get("X-User-Roles")+",", ",platform_super,") {
			common.JsonError(w, common.ErrForbidden)
			return
		}
		if write {
			var req model.ServiceIntentionUpdate
			if e := httpx.Parse(r, &req); e != nil {
				common.JsonError(w, common.ErrParam)
				return
			}
			if e := model.UpdateServiceIntentions(r.Context(), s.DB, req); e != nil {
				common.JsonError(w, e)
				return
			}
			common.Ok(w, map[string]bool{"updated": true})
			return
		}
		rows, e := model.ListServiceIntentions(r.Context(), s.DB)
		if e != nil {
			common.JsonError(w, common.ErrSystem)
			return
		}
		common.Ok(w, map[string]interface{}{"list": rows})
	}
}
