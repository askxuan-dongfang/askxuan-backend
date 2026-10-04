package handler

import (
	"encoding/json"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/common"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWeKnoraAdminBoundary(t *testing.T) {
	for _, action := range []string{"list", "create", "update", "delete", "documents", "manual", "upload", "chunks", "policy", "delete_doc", "reparse", "search", "wiki_status", "wiki_read", "wiki_models", "wiki_config", "wiki_write", "models", "model_create", "model_update", "model_delete", "model_test", "graph", "graph_config"} {
		for _, typ := range []string{"", "user", "master", "temple", "admin"} {
			r := httptest.NewRequest("POST", "/", strings.NewReader(`{}`))
			r.Header.Set("X-User-Id", "7")
			r.Header.Set("X-User-Type", typ)
			r.Header.Set("X-User-Roles", "customer")
			w := httptest.NewRecorder()
			weknoraHandler(&svc.ServiceContext{}, action)(w, r)
			var b common.Body
			_ = json.Unmarshal(w.Body.Bytes(), &b)
			if b.Code != common.ErrRoleForbidden.Code {
				t.Fatalf("%s %s bypassed permission: %s", typ, action, w.Body.String())
			}
		}
	}
}
