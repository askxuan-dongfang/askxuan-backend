package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/askxuan/ai-service/internal/catalogjob"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/askxuan/common/adminpolicy"
	catalog "github.com/askxuan/common/catalogtranslation"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func registerCatalogTranslation(server *rest.Server, s *svc.ServiceContext) {
	server.AddRoute(rest.Route{Method: http.MethodGet, Path: "/api/v1/ai/catalog-translations", Handler: catalogTranslationPublic(s)})
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut} {
		server.AddRoute(rest.Route{Method: method, Path: adminpolicy.CatalogTranslationPrefix + ":kind/:id", Handler: catalogTranslationManage(s)})
	}
}

func catalogTranslationPublic(s *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		query := r.URL.Query()
		kind := query.Get("kind")
		switch kind {
		case "points-product", "reward-campaign", "reward-prize":
			if r.Header.Get("X-User-Id") == "" {
				common.JsonError(w, common.ErrUnauthorized)
				return
			}
			if r.Header.Get("X-User-Type") != "user" || !slices.Contains(strings.Split(r.Header.Get("X-User-Roles"), ","), "customer") {
				common.JsonError(w, common.ErrRoleForbidden)
				return
			}
		}
		items, err := catalog.LookupSession(r.Context(), s.DB, kind, strings.Split(query.Get("ids"), ","))
		if err != nil {
			if errors.Is(err, catalog.ErrLookup) {
				common.JsonError(w, common.ErrParamInvalid)
			} else {
				common.JsonError(w, common.ErrDependencyUnavailable)
			}
			return
		}
		common.Ok(w, struct {
			Items map[string]catalog.Published `json:"items"`
		}{Items: items})
	}
}

func catalogTranslationManage(s *svc.ServiceContext) http.HandlerFunc {
	store := catalogjob.Store{DB: s.DB}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		h := r.Header
		clientID := h.Get("X-Client-Id")
		actor, err := strconv.ParseInt(h.Get("X-User-Id"), 10, 64)
		// The gateway authenticates the live session and strips client identity headers.
		if err != nil || actor <= 0 || !adminpolicy.CatalogTranslationAccess(r.Method, r.URL.Path, h.Get("X-User-Type"),
			clientID, h.Get("X-Authz-Scope"), strings.Split(h.Get("X-User-Roles"), ","), strings.Split(h.Get("X-User-Permissions"), ",")) {
			common.JsonError(w, common.ErrRoleForbidden)
			return
		}
		templeCode := ""
		if clientID == "temple-admin" {
			templeCode = h.Get("X-Temple-Code")
			if templeCode == "" {
				common.JsonError(w, common.ErrTempleIsolation)
				return
			}
		}
		var target struct {
			Kind string `path:"kind"`
			ID   string `path:"id"`
		}
		if httpx.ParsePath(r, &target) != nil {
			common.JsonError(w, common.ErrParamInvalid)
			return
		}
		var edit *catalogjob.Edit
		if r.Method == http.MethodPut {
			edit = &catalogjob.Edit{}
			decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 512<<10))
			decoder.DisallowUnknownFields()
			var extra any
			if decoder.Decode(edit) != nil || decoder.Decode(&extra) != io.EOF {
				common.JsonError(w, common.ErrParamInvalid)
				return
			}
		}
		draft, err := store.Manage(r.Context(), target.Kind, target.ID, templeCode, actor, r.Method == http.MethodPost, edit)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, draft)
	}
}
