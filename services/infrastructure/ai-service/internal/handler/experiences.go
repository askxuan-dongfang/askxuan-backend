package handler

import (
	"encoding/json"
	"github.com/askxuan/ai-service/internal/experience"
	"github.com/askxuan/ai-service/internal/logic"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
	"io"
	"net/http"
	"strconv"
)

func decodeExperience(w http.ResponseWriter, r *http.Request, v interface{}) error {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil {
		return common.ErrParamInvalid
	}
	if d.Decode(new(interface{})) != io.EOF {
		return common.ErrParamInvalid
	}
	return nil
}
func registerExperiences(server *rest.Server, s *svc.ServiceContext) {
	for _, op := range []struct{ method, path, action string }{{"POST", "/experiences/run", "run"}, {"POST", "/notes", "save"}, {"GET", "/notes", "list"}, {"GET", "/notes/:id", "get"}, {"DELETE", "/notes/:id", "delete"}} {
		server.AddRoute(rest.Route{Method: op.method, Path: "/api/v1/ai" + op.path, Handler: experienceHandler(s, op.action)})
	}
}
func experienceHandler(s *svc.ServiceContext, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		user, err := resolveUserID(r, "")
		if err != nil || r.Header.Get("X-User-Type") != "user" {
			respond(w, nil, common.ErrForbidden)
			return
		}
		var data interface{}
		switch action {
		case "run":
			var req experience.Input
			if err = decodeExperience(w, r, &req); err == nil {
				data, err = experience.Run(req)
				if err != nil {
					err = common.NewBizError(40001, err.Error())
				}
			}
		case "save":
			var req logic.ExperienceSave
			if err = decodeExperience(w, r, &req); err == nil {
				data, err = logic.ExperienceSaveNote(r.Context(), s, user, req)
			}
		case "list":
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			data, err = logic.ExperienceListNotes(r.Context(), s, user, page)
		default:
			var p struct {
				ID int64 `path:"id"`
			}
			if httpx.ParsePath(r, &p) != nil || p.ID <= 0 {
				err = common.ErrParamInvalid
			} else if action == "get" {
				data, err = logic.ExperienceGetNote(r.Context(), s, user, p.ID)
			} else {
				err = logic.ExperienceDeleteNote(r.Context(), s, user, p.ID)
				data = map[string]bool{"deleted": err == nil}
			}
		}
		respond(w, data, err)
	}
}
