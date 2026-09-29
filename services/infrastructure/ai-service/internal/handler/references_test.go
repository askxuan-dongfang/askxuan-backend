package handler

import (
	"encoding/json"
	"github.com/askxuan/ai-service/internal/knowledge"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/common"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReferenceAuthorizationAndConfirmation(t *testing.T) {
	for _, action := range []string{"list", "save", "search", "delete", "import"} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(`{}`))
		r.Header.Set("X-User-Id", "17")
		r.Header.Set("X-User-Type", "user")
		r.Header.Set("X-User-Roles", "customer")
		w := httptest.NewRecorder()
		referenceHandler(&svc.ServiceContext{}, "knowledge", action)(w, r)
		var b common.Body
		json.Unmarshal(w.Body.Bytes(), &b)
		if b.Code != common.ErrRoleForbidden.Code {
			t.Fatalf("%s allowed: %s", action, w.Body.String())
		}
	}
	for _, body := range []string{`{"entry":{"title":"偏好","text":"内容"},"confirmed":false}`, `{"entry":{"owner":"another","title":"偏好","text":"内容"},"confirmed":true}`} {
		r := httptest.NewRequest("POST", "/", strings.NewReader(body))
		r.Header.Set("X-User-Id", "17")
		w := httptest.NewRecorder()
		referenceHandler(&svc.ServiceContext{Knowledge: &knowledge.Store{}}, "memory", "save")(w, r)
		var b common.Body
		json.Unmarshal(w.Body.Bytes(), &b)
		if b.Code == 0 {
			t.Fatalf("unconfirmed or injected identity allowed: %s", w.Body.String())
		}
	}
}
