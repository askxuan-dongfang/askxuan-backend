package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/askxuan/audit-service/internal/model"
	"github.com/askxuan/audit-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/golang-jwt/jwt/v5"
)

const operationTestSecret = "isolated-operation-handler-test-secret-20261022"

type operationTestStore struct {
	starts, finishes, reads int
	failure                 error
	filter                  model.OperationFilter
}

func (s *operationTestStore) Start(_ context.Context, req model.OperationStart) (model.OperationReceipt, error) {
	s.starts++
	return model.OperationReceipt{RequestID: req.RequestID, Result: "pending"}, s.failure
}
func (s *operationTestStore) Finish(_ context.Context, req model.OperationFinish) (model.OperationReceipt, error) {
	s.finishes++
	return model.OperationReceipt{RequestID: req.RequestID, Result: req.Result}, s.failure
}
func (s *operationTestStore) List(_ context.Context, f model.OperationFilter) (model.OperationList, error) {
	s.reads++
	s.filter = f
	return model.OperationList{List: []model.OperationLog{}, Page: f.Page, Size: f.Size}, s.failure
}

func operationTestContext() (*svc.ServiceContext, *operationTestStore) {
	store := &operationTestStore{}
	ctx := &svc.ServiceContext{Operations: store}
	ctx.Config.Auth.AccessSecret = operationTestSecret
	return ctx, store
}

func operationToken(t *testing.T, info common.TokenInfo) string {
	t.Helper()
	token, err := common.GenAccessToken(operationTestSecret, info, 60)
	if err != nil {
		t.Fatal(err)
	}
	return token
}

func operationCall(t *testing.T, handler http.HandlerFunc, method, url, body, token string) (common.Body, string) {
	t.Helper()
	r := httptest.NewRequest(method, url, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	// These are never a substitute for a signed service/human identity.
	r.Header.Set("X-User-Roles", "platform_super,platform_service")
	r.Header.Set("X-User-Id", "7")
	w := httptest.NewRecorder()
	handler(w, r)
	var response common.Body
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response, w.Body.String()
}

const validOperationBody = `{"requestId":"0123456789abcdef0123456789abcdef","actorId":7,"actorRoles":["platform_super"],"action":"platform.products.write","method":"PUT","routeTemplate":"/api/v1/admin/products/:id","targetType":"products","targetId":"17"}`

func TestOperationInternalIdentityAndPayloadGuards(t *testing.T) {
	ctx, store := operationTestContext()
	service, err := common.SignServiceToken(operationTestSecret, "gateway-service")
	if err != nil {
		t.Fatal(err)
	}
	other, err := common.SignServiceToken(operationTestSecret, "order-service")
	if err != nil {
		t.Fatal(err)
	}
	refresh, err := common.GenRefreshToken(operationTestSecret, common.TokenInfo{UserId: 7, UserType: "admin"}, 60)
	if err != nil {
		t.Fatal(err)
	}
	wrongIssuerClaims := common.CustomClaims{UserType: "service", ClientID: "gateway-service", Type: "access", Roles: []string{"platform_service"}, RegisteredClaims: jwt.RegisteredClaims{Issuer: "order-service", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))}}
	wrongIssuer, err := jwt.NewWithClaims(jwt.SigningMethodHS256, wrongIssuerClaims).SignedString([]byte(operationTestSecret))
	if err != nil {
		t.Fatal(err)
	}
	for name, token := range map[string]string{
		"missing token": "", "invalid token": "invalid", "other service": other, "refresh": refresh, "wrong issuer": wrongIssuer,
		"human super":        operationToken(t, common.TokenInfo{UserId: 7, UserType: "admin", Roles: []string{"platform_super"}}),
		"user spoofing role": operationToken(t, common.TokenInfo{UserId: 7, UserType: "user", Roles: []string{"platform_service"}, ClientID: "gateway-service"}),
	} {
		t.Run(name, func(t *testing.T) {
			resp, _ := operationCall(t, operationStartHandler(ctx), http.MethodPost, "/api/v1/internal/admin-operations/start", validOperationBody, token)
			if resp.Code == 0 {
				t.Fatal("untrusted caller accepted")
			}
		})
	}
	if store.starts != 0 {
		t.Fatal("unauthorized requests reached storage")
	}
	for name, body := range map[string]string{
		"unknown body":    strings.TrimSuffix(validOperationBody, "}") + `,"body":{"password":"private"}}`,
		"unknown ip":      strings.TrimSuffix(validOperationBody, "}") + `,"ip":"127.0.0.1"}`,
		"trailing object": validOperationBody + ` {}`, "oversize": strings.Repeat(" ", 8193) + validOperationBody,
		"raw concrete path": strings.Replace(validOperationBody, "/api/v1/admin/products/:id", "/api/v1/admin/products/17", 1),
		"email identifier":  strings.Replace(validOperationBody, `"targetId":"17"`, `"targetId":"private@example.test"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			resp, _ := operationCall(t, operationStartHandler(ctx), http.MethodPost, "/api/v1/internal/admin-operations/start", body, service)
			if resp.Code != common.ErrParam.Code {
				t.Fatalf("unsafe body code=%d", resp.Code)
			}
		})
	}
	resp, _ := operationCall(t, operationStartHandler(ctx), http.MethodPost, "/api/v1/internal/admin-operations/start?password=private", validOperationBody, service)
	if resp.Code != common.ErrParam.Code || store.starts != 0 {
		t.Fatal("invalid bodies/queries reached storage")
	}
	resp, raw := operationCall(t, operationStartHandler(ctx), http.MethodPost, "/api/v1/internal/admin-operations/start", validOperationBody, service)
	if resp.Code != 0 || store.starts != 1 || !strings.Contains(raw, `"result":"pending"`) {
		t.Fatal("valid start rejected")
	}
	finish := `{"requestId":"0123456789abcdef0123456789abcdef","result":"denied","httpStatus":200,"businessCode":40301,"durationMs":5}`
	resp, _ = operationCall(t, operationFinishHandler(ctx), http.MethodPost, "/api/v1/internal/admin-operations/finish", finish, service)
	if resp.Code != 0 || store.finishes != 1 {
		t.Fatal("valid finalize rejected")
	}
	resp, _ = operationCall(t, operationFinishHandler(ctx), http.MethodPost, "/api/v1/internal/admin-operations/finish", strings.TrimSuffix(finish, "}")+`,"token":"private"}`, service)
	if resp.Code != common.ErrParam.Code || store.finishes != 1 {
		t.Fatal("unknown finalize metadata reached storage")
	}
}

func TestOperationReadUsesHumanJWTCapability(t *testing.T) {
	ctx, store := operationTestContext()
	service, _ := common.SignServiceToken(operationTestSecret, "gateway-service")
	cases := []struct {
		name  string
		info  common.TokenInfo
		allow bool
	}{
		{"super", common.TokenInfo{UserId: 7, UserType: "admin", Roles: []string{"platform_super"}}, true},
		{"delegated reader", common.TokenInfo{UserId: 8, UserType: "admin", ClientID: "platform-admin", AuthzScope: "platform", Permissions: []string{"platform.operations.read"}, Roles: []string{"auditor"}}, true},
		{"no permission", common.TokenInfo{UserId: 8, UserType: "admin", ClientID: "platform-admin", AuthzScope: "platform", Roles: []string{"auditor"}}, false},
		{"temple scope", common.TokenInfo{UserId: 8, UserType: "admin", ClientID: "temple-admin", AuthzScope: "temple", Roles: []string{"temple_admin"}, Permissions: []string{"platform.operations.read"}}, false},
		{"wrong client", common.TokenInfo{UserId: 8, UserType: "admin", ClientID: "master", AuthzScope: "platform", Roles: []string{"auditor"}, Permissions: []string{"platform.operations.read"}}, false},
		{"user", common.TokenInfo{UserId: 8, UserType: "user", Roles: []string{"platform_super"}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := store.reads
			resp, _ := operationCall(t, operationListHandler(ctx), http.MethodGet, operationListPath, "", operationToken(t, tc.info))
			if (resp.Code == 0) != tc.allow || store.reads-before != boolInt(tc.allow) {
				t.Fatalf("authorization result=%d reads=%d", resp.Code, store.reads-before)
			}
		})
	}
	before := store.reads
	for _, token := range []string{"", service} {
		resp, _ := operationCall(t, operationListHandler(ctx), http.MethodGet, operationListPath, "", token)
		if resp.Code == 0 {
			t.Fatal("missing/service identity allowed to read")
		}
	}
	if store.reads != before {
		t.Fatal("forbidden reads reached storage")
	}
	admin := operationToken(t, cases[0].info)
	for _, query := range []string{"?email=private", "?page=1&page=2", "?size=101", "?page=0", "?actorId=bad", "?result=complete", "?startTime=bad", "?endTime=2026-10-21T00:00:00Z&startTime=2026-10-22T00:00:00Z", "?page=1;size=20"} {
		resp, _ := operationCall(t, operationListHandler(ctx), http.MethodGet, operationListPath+query, "", admin)
		if resp.Code != common.ErrParam.Code {
			t.Fatalf("invalid filter accepted: %s", query)
		}
	}
	if store.reads != before {
		t.Fatal("invalid filters reached storage")
	}
	resp, _ := operationCall(t, operationListHandler(ctx), http.MethodGet, operationListPath+"?actorId=7&page=2&size=1&result=denied&startTime=2026-10-22T00:00:00Z", "", admin)
	if resp.Code != 0 || store.filter.ActorID != 7 || store.filter.Page != 2 || store.filter.Size != 1 || store.filter.Result != "denied" {
		t.Fatal("valid filters lost")
	}
	store.failure = errors.New("private-database-credential")
	resp, raw := operationCall(t, operationListHandler(ctx), http.MethodGet, operationListPath, "", admin)
	if resp.Code != common.ErrSystem.Code || strings.Contains(raw, "private-database-credential") {
		t.Fatal("storage errors leaked")
	}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
