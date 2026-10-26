package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/askxuan/ai-service/internal/logic"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// Same private-service trust boundary as provider settings: the gateway validates
// live admin JWTs and overwrites these identity headers; service ports are private.
func reportPricingAdmin(r *http.Request) bool {
	id, err := strconv.ParseInt(r.Header.Get("X-User-Id"), 10, 64)
	if err != nil || id <= 0 || r.Header.Get("X-User-Type") != "admin" {
		return false
	}
	for _, role := range strings.Split(r.Header.Get("X-User-Roles"), ",") {
		if role == "platform_super" {
			return true
		}
	}
	return false
}

func registerReportPrices(server *rest.Server, s *svc.ServiceContext) {
	for _, op := range []struct{ method, path string }{{"GET", "/api/v1/ai/admin/products"}, {"PUT", "/api/v1/ai/admin/products/:code"}} {
		server.AddRoute(rest.Route{Method: op.method, Path: op.path, Handler: reportPriceHandler(s)})
	}
}
func reportPriceHandler(s *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if !reportPricingAdmin(r) {
			common.JsonError(w, common.ErrRoleForbidden)
			return
		}
		var out any
		var err error
		if r.Method == http.MethodGet {
			out, err = logic.ReportPriceProducts(r.Context(), s.DB)
		} else {
			var p struct {
				Code string `path:"code"`
			}
			var req logic.ReportPriceUpdate
			decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
			decoder.DisallowUnknownFields()
			var extra any
			if httpx.ParsePath(r, &p) != nil || decoder.Decode(&req) != nil || decoder.Decode(&extra) != io.EOF {
				common.JsonError(w, common.ErrParam)
				return
			}
			out, err = logic.UpdateReportPrice(r.Context(), s.DB, r.Header.Get("X-User-Id"), p.Code, req)
		}
		if err != nil {
			if _, ok := err.(*common.BizError); !ok {
				err = common.ErrSystem
			}
			common.JsonError(w, err)
			return
		}
		respond(w, out, nil)
	}
}
