package handler

import (
	"io"
	"net/http"
	"strings"

	"github.com/askxuan/common"
	"github.com/askxuan/file-service/internal/logic"
	"github.com/askxuan/file-service/internal/svc"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func requireDiyService(w http.ResponseWriter, r *http.Request, secret string) bool {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		c, e := common.ParseToken(secret, parts[1])
		if e == nil && c.Type == "access" && c.UserType == "service" && c.UserId == 0 && c.Issuer == "diy-service" && c.ClientID == "diy-service" && c.HasRole("platform_service") {
			return true
		}
	}
	common.JsonError(w, common.ErrRoleForbidden)
	return false
}
func registerModelAssets(server *rest.Server, s *svc.ServiceContext) {
	server.AddRoute(rest.Route{Method: http.MethodPut, Path: "/internal/diy-models/:id/:format", Handler: func(w http.ResponseWriter, r *http.Request) {
		if !requireDiyService(w, r, s.Config.AuthSecret) {
			return
		}
		var p struct {
			ID     string `path:"id"`
			Format string `path:"format"`
		}
		if httpx.ParsePath(r, &p) != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		body, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 32<<20))
		if e != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		out, e := logic.StoreModel(r.Context(), s, p.ID, p.Format, body)
		if e != nil {
			common.JsonError(w, e)
			return
		}
		common.Ok(w, out)
	}}, rest.WithMaxBytes(32<<20))
	server.AddRoute(rest.Route{Method: http.MethodGet, Path: "/internal/diy-model-photos", Handler: func(w http.ResponseWriter, r *http.Request) {
		if !requireDiyService(w, r, s.Config.AuthSecret) {
			return
		}
		data, mime, e := logic.ModelingPhoto(r.Context(), s, r.URL.Query().Get("url"))
		if e != nil {
			common.JsonError(w, e)
			return
		}
		w.Header().Set("Content-Type", mime)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = w.Write(data)
	}})
}
