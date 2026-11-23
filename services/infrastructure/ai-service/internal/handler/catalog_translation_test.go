package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/common"
)

func TestCatalogTranslationCommerceReadRequiresCustomer(t *testing.T) {
	handler := catalogTranslationPublic(&svc.ServiceContext{})
	for _, kind := range []string{"points-product", "reward-campaign", "reward-prize"} {
		for _, admin := range []bool{false, true} {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/ai/catalog-translations?kind="+kind+"&ids=1", nil)
			want := common.ErrUnauthorized.Code
			if admin {
				req.Header.Set("X-User-Id", "1")
				req.Header.Set("X-User-Type", "admin")
				req.Header.Set("X-User-Roles", "platform_super")
				want = common.ErrRoleForbidden.Code
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			var body common.Body
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || body.Code != want {
				t.Fatalf("%s customer authorization: got %s want %d", kind, response.Body.String(), want)
			}
		}
	}
}
