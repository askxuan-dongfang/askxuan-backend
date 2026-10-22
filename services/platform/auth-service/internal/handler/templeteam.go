package handler

import (
	"encoding/json"
	"github.com/askxuan/auth-service/internal/logic"
	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/rest"
	"net/http"
)

func registerTempleTeam(server *rest.Server, s *svc.ServiceContext) {
	for _, method := range []string{"GET", "POST", "PUT"} {
		method := method
		server.AddRoute(rest.Route{Method: method, Path: "/api/v1/auth/temple/team", Handler: func(w http.ResponseWriter, r *http.Request) {
			c, _, err := currentAdminClaims(s, r)
			if err != nil {
				common.JsonError(w, err)
				return
			}
			if !c.HasRole("temple_admin") || c.TempleCode == "" {
				common.JsonError(w, common.ErrForbidden)
				return
			}
			if method == "GET" {
				rows, e := logic.TempleTeamList(r.Context(), s, c)
				if e != nil {
					common.JsonError(w, common.ErrSystem)
					return
				}
				common.Ok(w, map[string]any{"list": rows})
				return
			}
			var req logic.TempleTeamInput
			d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
			d.DisallowUnknownFields()
			if d.Decode(&req) != nil || (method == "POST" && req.ID != 0) || (method == "PUT" && req.ID <= 0) {
				common.JsonError(w, common.ErrParam)
				return
			}
			id, e := logic.SaveTempleTeam(r.Context(), s, c, req)
			if e != nil {
				if _, ok := e.(*common.BizError); ok {
					common.JsonError(w, e)
				} else {
					common.JsonError(w, common.ErrSystem)
				}
				return
			}
			common.Ok(w, map[string]any{"id": id})
		}})
	}
}
