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

func TestCustomerAIDataRejectsSameNumericAdministrator(t *testing.T) {
	for _, path := range []string{"/api/v1/ai/memory/search", "/api/v1/ai/sessions/7/messages", "/api/v1/ai/reports/7"} {
		r := httptest.NewRequest("GET", path, nil)
		r.Header.Set("X-User-Id", "7")
		r.Header.Set("X-User-Type", "admin")
		r.Header.Set("X-User-Roles", "master,platform_super")
		if _, e := resolveUserID(r, ""); e == nil {
			t.Fatal("cross-domain identity accepted", path)
		}
		r.Header.Set("X-User-Type", "user")
		if id, e := resolveUserID(r, ""); e != nil || id != "7" {
			t.Fatal("customer rejected", path)
		}
	}
}
