package handler

import (
	"github.com/askxuan/ai-service/internal/config"
	"github.com/askxuan/ai-service/internal/settings"
	"github.com/askxuan/ai-service/internal/svc"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProviderAdminSettingsRoleAndRedaction(t *testing.T) {
	m, err := settings.New(config.AIConf{Provider: "mock", APIKey: "secret-fixture"}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		kind, roles string
		allowed     bool
	}{{"user", "customer", false}, {"admin", "shop_admin", false}, {"admin", "platform_service", false}, {"service", "platform_super", false}, {"admin", "platform_super", true}} {
		req := httptest.NewRequest("GET", "/api/v1/ai/admin/provider", nil)
		req.Header.Set("X-User-Id", "1")
		req.Header.Set("X-User-Type", tc.kind)
		req.Header.Set("X-User-Roles", tc.roles)
		out := httptest.NewRecorder()
		providerSettingsHandler(&svc.ServiceContext{Settings: m}, "get")(out, req)
		if strings.Contains(out.Body.String(), "secret-fixture") {
			t.Fatal("API key leaked")
		}
		if strings.Contains(out.Body.String(), `"hasApiKey":true`) != tc.allowed {
			t.Fatalf("bad access for %s/%s: %s", tc.kind, tc.roles, out.Body)
		}
		if out.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("sensitive config was cacheable")
		}
	}
}
