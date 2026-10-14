package handler

import (
	"encoding/json"
	"github.com/askxuan/product-service/internal/svc"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServiceIntentionsAuthorizationAndMalformedWrites(t *testing.T) {
	for _, tc := range []struct {
		name, user, role, id, body string
		write                      bool
	}{
		{"anonymous", "", "", "", `{}`, false},
		{"customer cannot write", "customer", "platform_super", "1", `{}`, true},
		{"temple admin cannot write", "admin", "temple_admin", "1", `{}`, true},
		{"role substring rejected", "admin", "platform_super_fake", "1", `{}`, true},
		{"invalid identity rejected", "admin", "platform_super", "0", `{}`, false},
		{"malformed JSON", "admin", "platform_super", "1", `{`, true},
		{"unknown wish", "admin", "platform_super", "1", `{"kind":"rule","serviceCode":"S001","tags":["diy"]}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("X-User-Type", tc.user)
			r.Header.Set("X-User-Roles", tc.role)
			r.Header.Set("X-User-Id", tc.id)
			w := httptest.NewRecorder()
			serviceIntentions(&svc.ServiceContext{}, tc.write)(w, r)
			var result struct {
				Code int `json:"code"`
			}
			if e := json.Unmarshal(w.Body.Bytes(), &result); e != nil {
				t.Fatal(e)
			}
			if result.Code == 0 {
				t.Fatal("unauthorized or malformed input accepted", w.Body.String())
			}
		})
	}
}
