package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/askxuan/common"
	"github.com/askxuan/common/adminpolicy"
)

type auditFixture struct {
	startErr, finishErr  error
	starts               []OperationStart
	finishes             []OperationFinish
	finalContextCanceled bool
}

func (f *auditFixture) Start(_ context.Context, e OperationStart) error {
	f.starts = append(f.starts, e)
	return f.startErr
}
func (f *auditFixture) Finish(ctx context.Context, e OperationFinish) error {
	f.finishes = append(f.finishes, e)
	f.finalContextCanceled = ctx.Err() != nil
	return f.finishErr
}
func adminRequest(t *testing.T, method, path string, perms []string) *http.Request {
	t.Helper()
	token, err := common.GenAccessToken("gateway-security-fixture", common.TokenInfo{UserId: 7, UserType: "admin", Roles: []string{"fixture_editor"}, ClientID: "platform-admin", AuthzScope: "platform", Permissions: perms}, 60)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(method, path, strings.NewReader(`{"password":"private-password-sentinel","email":"private-email@example.invalid","secret":"private-key-sentinel"}`))
	r.Header.Set("Authorization", "Bearer "+token)
	return r
}
func TestGatewayFiniteCustomPermissionsAndSuperOnlyBoundaries(t *testing.T) {
	all := []string{}
	for _, c := range adminpolicy.Catalog {
		if c.Assignable {
			all = append(all, c.Code)
		}
	}
	for _, route := range adminpolicy.Routes {
		if !adminpolicy.Assignable(route.Permission) {
			continue
		}
		t.Run(route.Method+route.Template, func(t *testing.T) {
			for _, granted := range []bool{false, true} {
				calls := 0
				perms := []string{}
				if granted {
					perms = []string{route.Permission}
				}
				handler := AuthWithOptions("gateway-security-fixture", nil, AuthOptions{})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
				r := adminRequest(t, route.Method, route.Template, perms)
				r.Header.Set("X-User-Permissions", strings.Join(all, ","))
				r.Header.Set("X-User-Roles", "platform_super")
				handler.ServeHTTP(httptest.NewRecorder(), r)
				if (calls == 1) != granted {
					t.Fatalf("grant=%v reached=%d", granted, calls)
				}
			}
		})
	}
	for _, route := range []struct{ method, path string }{{"POST", "/api/v1/admin/auth/roles"}, {"PUT", "/api/v1/admin/platform/beliefs/han_buddhism"}, {"POST", "/api/v1/admin/finance/payout-sandbox"}, {"GET", "/api/v1/admin/platform/service-orders/bookings"}, {"POST", "/api/v1/admin/diy/creator-withdrawals/1/action"}, {"PUT", "/api/v1/admin/orders/returns/1/refund"}, {"PUT", "/api/v1/admin/platform/masters/1"}} {
		calls := 0
		h := AuthWithOptions("gateway-security-fixture", nil, AuthOptions{})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
		h.ServeHTTP(httptest.NewRecorder(), adminRequest(t, route.method, route.path, all))
		if calls != 0 {
			t.Fatalf("root-only route delegated %s", route.path)
		}
	}
}
func TestAuditFailureBoundariesAndBusinessResults(t *testing.T) {
	for _, tc := range []struct {
		name          string
		start, finish bool
		code, status  int
		result        string
	}{
		{"start fails", true, false, 0, 200, ""}, {"success finalization fails", false, true, 0, 200, "success"}, {"business fails", false, false, 40903, 200, "failed"}, {"denied", false, false, common.ErrRoleForbidden.Code, 200, "denied"},
		{"provider isolation denied", false, false, common.ErrTempleIsolation.Code, 200, "denied"}, {"downstream HTTP fail", false, false, 0, 502, "failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := &auditFixture{}
			if tc.start {
				rec.startErr = errors.New("unavailable")
			}
			if tc.finish {
				rec.finishErr = errors.New("unavailable")
			}
			calls := 0
			h := AuthWithOptions("gateway-security-fixture", nil, AuthOptions{Recorder: rec})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(`{"code":` + fmtInt(tc.code) + `,"data":{"submitted":true}}`))
			}))
			w := httptest.NewRecorder()
			h.ServeHTTP(w, adminRequest(t, "PUT", "/api/v1/admin/products/42?email=private-query@example.invalid", []string{"platform.products.write"}))
			if tc.start {
				if calls != 0 || len(rec.finishes) != 0 {
					t.Fatal("write executed without durable audit start")
				}
				return
			}
			if calls != 1 || len(rec.starts) != 1 || len(rec.finishes) != 1 || rec.finishes[0].Result != tc.result {
				t.Fatalf("calls=%d events=%+v", calls, rec.finishes)
			}
			if w.Code != tc.status || !strings.Contains(w.Body.String(), `"submitted":true`) {
				t.Fatalf("audit changed business result %d %s", w.Code, w.Body.String())
			}
			if (w.Header().Get("X-Admin-Audit-Pending") == "true") != tc.finish {
				t.Fatal("missing or spurious audit pending marker")
			}
			data, _ := json.Marshal(map[string]any{"starts": rec.starts, "finishes": rec.finishes})
			for _, secret := range []string{"private-password", "private-email", "private-key", "private-query", "example.invalid"} {
				if strings.Contains(string(data), secret) {
					t.Fatalf("sensitive request leaked into audit %s", secret)
				}
			}
			if rec.starts[0].RouteTemplate != "/api/v1/admin/products/:id" || rec.starts[0].TargetID != "42" {
				t.Fatalf("unsafe/missing target %+v", rec.starts[0])
			}
		})
	}
}
func fmtInt(n int) string { b, _ := json.Marshal(n); return string(b) }
func TestDeniedAdminWritesAuditedButUnsignedAndServiceNotRecorded(t *testing.T) {
	rec := &auditFixture{}
	calls := 0
	h := AuthWithOptions("gateway-security-fixture", nil, AuthOptions{Recorder: rec})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
	h.ServeHTTP(httptest.NewRecorder(), adminRequest(t, "POST", "/api/v1/admin/products", nil))
	if calls != 0 || len(rec.finishes) != 1 || rec.finishes[0].Result != "denied" {
		t.Fatal("verified denial not recorded")
	}
	bad := httptest.NewRequest("POST", "/api/v1/admin/products", nil)
	bad.Header.Set("X-User-Id", "7")
	bad.Header.Set("X-User-Roles", "platform_super")
	h.ServeHTTP(httptest.NewRecorder(), bad)
	service, _ := common.SignServiceToken("gateway-security-fixture", "fixture-service")
	req := httptest.NewRequest("POST", "/api/v1/admin/products", nil)
	req.Header.Set("Authorization", "Bearer "+service)
	h.ServeHTTP(httptest.NewRecorder(), req)
	if len(rec.starts) != 1 || calls != 1 {
		t.Fatal("unsigned/service calls logged as a human actor")
	}
}
func TestEveryAdminSurfaceFailsClosedOnRevokedAccess(t *testing.T) {
	for _, path := range []string{"/api/v1/admin/products", "/api/v1/admin/platform/service-orders/bookings/1/fulfillment", "/api/v1/admin/bookings/1/receipt-files/2/download", "/api/v1/auth/access", "/api/v1/ai/admin/provider", "/api/v1/payments/growth/admin/rules"} {
		checks, calls := 0, 0
		h := AuthWithOptions("gateway-security-fixture", nil, AuthOptions{ValidateAccess: func(context.Context, string, *common.CustomClaims) error { checks++; return common.ErrTokenInvalid }})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, adminRequest(t, "GET", path, nil))
		if checks != 1 || calls != 0 {
			t.Fatalf("revoked access reached %s", path)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestAccessIntrospectionIsDirectBoundedAndDoesNotForwardIdentityHeaders(t *testing.T) {
	calls := 0
	s := &SecurityClient{AuthURL: "http://auth.internal:8081", Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "auth.internal:8081" || r.URL.Path != "/api/v1/auth/access" || r.Method != "GET" || r.Header.Get("X-User-Roles") != "" {
			t.Fatalf("recursive/unsafe introspection %+v", r)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"code":0,"data":{}}`)), Header: make(http.Header)}, nil
	})}}
	if err := s.ValidateAccess(context.Background(), "original-signed-token", &common.CustomClaims{UserType: "admin"}); err != nil || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
	s.Client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("unavailable") })
	if s.ValidateAccess(context.Background(), "token", &common.CustomClaims{UserType: "admin"}) == nil {
		t.Fatal("unavailable auth opened admin access")
	}
}
func TestAuditUnknownAndDetachedFinalization(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	rec := &auditFixture{}
	r := adminRequest(t, "PUT", "/api/v1/admin/products/4", []string{"platform.products.write"}).WithContext(ctx)
	h := AuthWithOptions("gateway-security-fixture", nil, AuthOptions{Recorder: rec})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		cancel()
		_, _ = w.Write([]byte("non-standard-response"))
	}))
	h.ServeHTTP(httptest.NewRecorder(), r)
	if len(rec.finishes) != 1 || rec.finishes[0].Result != "unknown" || rec.finishes[0].BusinessCode != -1 || rec.finalContextCanceled {
		t.Fatal("unknown response finalized as a known failure or canceled")
	}
}
func TestAuditTargetNeverCapturesFreeTextAndKeepsSandboxID(t *testing.T) {
	for _, id := range []string{"private@example.invalid", "free-text-secret", "sandbox_123e4567-e89b-12d3-a456-426614174000"} {
		r := httptest.NewRequest("POST", "/api/v1/admin/finance/payout-sandbox/"+id+"/actions", nil)
		e, err := operationFor(r, &common.CustomClaims{UserId: 7, UserType: "admin", Roles: []string{"platform_super"}})
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(id, "sandbox_") {
			if e.TargetID != id || e.Action != "super.sandbox" {
				t.Fatalf("sandbox target lost %+v", e)
			}
		} else if e.TargetID != "" {
			t.Fatal("free text captured")
		}
	}
}
