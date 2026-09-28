package handler

import (
	"net/http/httptest"
	"strings"
	"testing"

	"encoding/json"
	"github.com/askxuan/ai-service/internal/agentops"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/common"
)

func TestAgentOperationsAdminGate(t *testing.T) {
	for _, action := range []string{"workspace", "save", "publish", "rollback", "debug-start", "debug-get", "debug-list", "debug-resume", "debug-cancel", "runs", "tools", "version-get"} {
		for _, role := range []struct{ kind, roles, id string }{{"user", "customer", "1"}, {"admin", "platform_service", "1"}, {"admin", "shop_admin", "1"}, {"master", "platform_super", "1"}, {"admin", "platform_super", "0"}, {"admin", "platform_super", ""}} {
			t.Run(action+"/"+role.kind+"/"+role.roles+"/"+role.id, func(t *testing.T) {
				r := httptest.NewRequest("POST", "/api/v1/ai/admin/agent", strings.NewReader(`{}`))
				r.Header.Set("X-User-Id", role.id)
				r.Header.Set("X-User-Type", role.kind)
				r.Header.Set("X-User-Roles", role.roles)
				w := httptest.NewRecorder()
				agentOperationsHandler(&svc.ServiceContext{AgentOps: &agentops.Manager{}}, action)(w, r)
				var body common.Body
				if json.Unmarshal(w.Body.Bytes(), &body) != nil || body.Code != common.ErrRoleForbidden.Code {
					t.Fatalf("admin gate bypass: %s", w.Body.String())
				}
				if w.Header().Get("Cache-Control") != "no-store" {
					t.Fatal("cacheable response")
				}
			})
		}
	}
}
