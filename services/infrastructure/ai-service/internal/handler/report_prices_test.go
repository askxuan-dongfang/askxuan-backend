package handler

import (
	"encoding/json"
	"github.com/askxuan/ai-service/internal/svc"
	"net/http/httptest"
	"testing"
)

func TestReportPriceAdministrationRejectsOtherIdentityDomains(t *testing.T) {
	for _, test := range []struct{ id, kind, roles string }{{"", "", ""}, {"1", "user", "platform_super"}, {"1", "service", "platform_super"}, {"1", "admin", "shop_admin"}, {"0", "admin", "platform_super"}} {
		r := httptest.NewRequest("GET", "/api/v1/ai/admin/products", nil)
		r.Header.Set("X-User-Id", test.id)
		r.Header.Set("X-User-Type", test.kind)
		r.Header.Set("X-User-Roles", test.roles)
		w := httptest.NewRecorder()
		reportPriceHandler(&svc.ServiceContext{})(w, r)
		var out struct {
			Code int `json:"code"`
		}
		if json.Unmarshal(w.Body.Bytes(), &out) != nil || out.Code == 0 {
			t.Fatal("pricing opened to non-super identity")
		}
	}
}
