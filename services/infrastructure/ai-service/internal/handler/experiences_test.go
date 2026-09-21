package handler

import (
	"encoding/json"
	"github.com/askxuan/ai-service/internal/svc"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExperienceIdentityAndStrictInput(t *testing.T) {
	for _, tt := range []struct {
		user, kind, body string
		ok               bool
	}{
		{"", "user", `{"skill":"naming","naming":{"purpose":"pen","style":"clear"}}`, false},
		{"7", "admin", `{"skill":"naming","naming":{"purpose":"pen","style":"clear"}}`, false},
		{"7", "user", `{"skill":"naming","naming":{"purpose":"pen","style":"clear"}}`, true},
		{"7", "user", `{"skill":"naming","naming":{"purpose":"pen","style":"clear"},"userId":"8"}`, false},
		{"7", "user", `{"skill":"naming","naming":{"purpose":"pen","style":"clear"}} {}`, false},
		{"7", "user", `{"skill":"decision","decision":{"a":"A","b":"B","factors":[{"label":"x","weight":1.5,"a":2,"b":2}]}}`, false},
	} {
		r := httptest.NewRequest("POST", "/api/v1/ai/experiences/run", strings.NewReader(tt.body))
		r.Header.Set("X-User-Id", tt.user)
		r.Header.Set("X-User-Type", tt.kind)
		w := httptest.NewRecorder()
		experienceHandler(&svc.ServiceContext{}, "run")(w, r)
		var body struct{ Code int }
		if json.Unmarshal(w.Body.Bytes(), &body) != nil {
			t.Fatal(w.Body.String())
		}
		if (body.Code == 0) != tt.ok {
			t.Fatalf("unexpected response %s", w.Body.String())
		}
	}
}
