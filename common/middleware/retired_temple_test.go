package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/askxuan/common"
)

func TestRetiredTempleSessionsCannotReachServices(t *testing.T) {
	for _, role := range []string{"temple_reception", "temple_operations", "temple_finance"} {
		for _, roles := range [][]string{{role}, {role, "temple_admin"}, {role, "platform_super"}} {
			token, err := common.GenAccessToken("retired-fixture", common.TokenInfo{UserId: 7, UserType: "admin", ClientID: "temple-admin", TempleID: 1, TempleCode: "T1", Roles: roles, Permissions: []string{"platform.products.write", "temple.finance.read"}}, 60)
			if err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{"/api/v1/admin/temples/info", "/api/v1/bookings/journeys", "/api/v1/auth/access"} {
				called := false
				next := (&AuthConfig{Secret: "retired-fixture"}).Auth(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
				req := httptest.NewRequest("GET", path, nil)
				req.Header.Set("Authorization", "Bearer "+token)
				response := httptest.NewRecorder()
				next.ServeHTTP(response, req)
				var body struct {
					Code int `json:"code"`
				}
				if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				if called || body.Code != common.ErrSessionRevoked.Code {
					t.Fatalf("retired session reached %s: roles=%v code=%d", path, roles, body.Code)
				}
			}
		}
	}
	for _, role := range []string{"temple_admin", "master"} {
		called := false
		token, _ := common.GenAccessToken("retired-fixture", common.TokenInfo{UserId: 7, UserType: "admin", Roles: []string{role}}, 60)
		req := httptest.NewRequest("GET", "/api/v1/bookings/journeys", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		(&AuthConfig{Secret: "retired-fixture"}).Auth(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })).ServeHTTP(httptest.NewRecorder(), req)
		if !called {
			t.Fatal("existing provider blocked", role)
		}
	}
}
