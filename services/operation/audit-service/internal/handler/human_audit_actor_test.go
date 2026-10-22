package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/askxuan/audit-service/internal/types"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/rest/pathvar"
)

func humanAuditCall(t *testing.T, handler http.HandlerFunc, method, path, body, token string) common.Body {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r = pathvar.WithVars(r, map[string]string{"id": "41"})
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	r.Header.Set("X-User-Id", "999")
	r.Header.Set("X-User-Roles", "platform_super")
	w := httptest.NewRecorder()
	handler(w, r)
	var response common.Body
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response
}

func TestManualAuditBindsVerifiedActorAndRejectsServiceCallers(t *testing.T) {
	ctx, _ := operationTestContext()
	super := operationToken(t, common.TokenInfo{UserId: 7, UserType: "admin", Roles: []string{"platform_super"}, ClientID: "platform-admin"})
	moderator := operationToken(t, common.TokenInfo{UserId: 8, UserType: "admin", Roles: []string{"moderator"}, ClientID: "platform-admin", AuthzScope: "platform", Permissions: []string{"platform.moderation.read", "platform.moderation.write"}})
	service, _ := common.SignServiceToken(operationTestSecret, "gateway-service")
	refresh, _ := common.GenRefreshToken(operationTestSecret, common.TokenInfo{UserId: 7, UserType: "admin"}, 60)
	cases := []struct {
		name  string
		token string
	}{
		{"missing token with forged headers", ""},
		{"invalid signature", "not-a-jwt"},
		{"service", service},
		{"refresh", refresh},
		{"customer with super role", operationToken(t, common.TokenInfo{UserId: 7, UserType: "user", Roles: []string{"platform_super"}})},
		{"zero human", operationToken(t, common.TokenInfo{UserType: "admin", Roles: []string{"platform_super"}})},
		{"temple", operationToken(t, common.TokenInfo{UserId: 7, UserType: "admin", Roles: []string{"temple_admin"}, ClientID: "temple-admin"})},
		{"moderator cannot enter onboarding queue", moderator},
	}
	for _, approve := range []bool{true, false} {
		calls := 0
		var actor string
		var handler http.HandlerFunc
		path := "/api/v1/admin/audit/queue/41/approve"
		if approve {
			handler = auditApproveWithAction(ctx, func(_ context.Context, req *types.AuditApproveReq) (*types.AuditApproveResp, error) {
				calls++
				actor = req.AuditorId
				return &types.AuditApproveResp{Id: req.Id, Status: "approved"}, nil
			})
		} else {
			path = "/api/v1/admin/audit/queue/41/reject"
			handler = auditRejectWithAction(ctx, func(_ context.Context, req *types.AuditRejectReq) (*types.AuditRejectResp, error) {
				calls++
				actor = req.AuditorId
				return &types.AuditRejectResp{Id: req.Id, Status: "rejected"}, nil
			})
		}
		for _, tc := range cases {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				response := humanAuditCall(t, handler, http.MethodPut, path, `{"auditorId":"999","remark":"review"}`, tc.token)
				if response.Code == 0 || calls != 0 {
					t.Fatal("untrusted manual review reached business action")
				}
			})
		}
		response := humanAuditCall(t, handler, http.MethodPut, path, `{"auditorId":"999","remark":"review"}`, super)
		if response.Code != 0 || calls != 1 || actor != "7" {
			t.Fatal("client-reported reviewer replaced the signed human identity")
		}
	}
}

func TestReportHandlingUsesExactDelegatedPermissionAndVerifiedActor(t *testing.T) {
	ctx, _ := operationTestContext()
	calls := 0
	var actor string
	handler := reportHandleWithAction(ctx, func(_ context.Context, req *types.ReportHandleReq) (*types.ReportHandleResp, error) {
		calls++
		actor = req.HandlerId
		return &types.ReportHandleResp{Id: req.Id, Status: "handled"}, nil
	})
	path := "/api/v1/admin/audit/reports/41/handle"
	body := `{"handlerId":"999","handleResult":"handled","remark":"review"}`
	base := common.TokenInfo{UserId: 8, UserType: "admin", ClientID: "platform-admin", AuthzScope: "platform", Roles: []string{"moderator"}, Permissions: []string{"platform.moderation.read", "platform.moderation.write"}}
	for _, change := range []func(*common.TokenInfo){
		func(c *common.TokenInfo) { c.Permissions = []string{"platform.moderation.read"} },
		func(c *common.TokenInfo) { c.ClientID = "temple-admin" },
		func(c *common.TokenInfo) { c.AuthzScope = "temple" },
		func(c *common.TokenInfo) { c.UserType = "service" },
		func(c *common.TokenInfo) { c.UserId = 0 },
	} {
		bad := base
		change(&bad)
		if response := humanAuditCall(t, handler, http.MethodPut, path, body, operationToken(t, bad)); response.Code == 0 || calls != 0 {
			t.Fatal("invalid delegated reviewer accepted")
		}
	}
	moderator := operationToken(t, base)
	for _, bad := range []struct{ method, path string }{
		{http.MethodPost, path}, {http.MethodPut, "/api/v1/admin/audit/reports/41/handle/extra"}, {http.MethodPut, "/api/v1/admin/audit/queue/41/approve"},
	} {
		if response := humanAuditCall(t, handler, bad.method, bad.path, body, moderator); response.Code == 0 || calls != 0 {
			t.Fatal("delegation authorized the wrong method/path")
		}
	}
	response := humanAuditCall(t, handler, http.MethodPut, path, body, moderator)
	if response.Code != 0 || calls != 1 || actor != "8" {
		t.Fatal("report actor was not bound to the authorized human")
	}
	super := operationToken(t, common.TokenInfo{UserId: 7, UserType: "admin", Roles: []string{"platform_super"}})
	response = humanAuditCall(t, handler, http.MethodPut, path, body, super)
	if response.Code != 0 || calls != 2 || actor != "7" {
		t.Fatal("human super lost report compatibility")
	}
}
