package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/askxuan/common"
	"github.com/askxuan/gateway-service/internal/config"
	"github.com/askxuan/gateway-service/internal/proxy"
)

func TestConsumerAvailabilityCoversReadsWritesUploadsAndPublicPersonalization(t *testing.T) {
	token, _ := common.GenAccessToken("consumer-security-test", common.TokenInfo{UserId: 7, UserType: "user", Roles: []string{"customer"}, ClientID: "customer"}, 60)
	for _, surface := range []struct{ method, path string }{
		{"GET", "/api/v1/users/profile"}, {"PUT", "/api/v1/users/profile"}, {"GET", "/api/v1/bookings"},
		{"POST", "/api/v1/files/upload"}, {"GET", "/api/v1/bookings/1/receipt-files/2/download"},
		{"POST", "/api/v1/ai/sessions/1/messages"}, {"POST", "/api/v1/auth/im-token"}, {"GET", "/api/v1/products"},
	} {
		t.Run(surface.method+surface.path, func(t *testing.T) {
			calls, checks := 0, 0
			h := AuthWithOptions("consumer-security-test", []string{"/api/v1/products"}, AuthOptions{ValidateAccess: func(context.Context, string, *common.CustomClaims) error { checks++; return common.ErrSessionRevoked }})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls++ }))
			req := httptest.NewRequest(surface.method, surface.path, nil)
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			var body struct {
				Code int `json:"code"`
			}
			_ = json.Unmarshal(w.Body.Bytes(), &body)
			if calls != 0 || checks != 1 || body.Code != 40105 {
				t.Fatalf("revoked consumer reached %s checks=%d calls=%d code=%d", surface.path, checks, calls, body.Code)
			}
		})
	}
}

func TestGuestAndInternalServiceRequestsDoNotAcquireAConsumerIdentity(t *testing.T) {
	checks, calls := 0, 0
	h := AuthWithOptions("consumer-security-test", []string{"/api/v1/products"}, AuthOptions{ValidateAccess: func(context.Context, string, *common.CustomClaims) error { checks++; return common.ErrSessionRevoked }})(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("X-User-Id") == "77" {
			t.Fatal("untrusted identity forwarded")
		}
	}))
	guest := httptest.NewRequest("GET", "/api/v1/products", nil)
	guest.Header.Set("X-User-Id", "77")
	h.ServeHTTP(httptest.NewRecorder(), guest)
	service, _ := common.SignServiceToken("consumer-security-test", "order-service")
	req := httptest.NewRequest("POST", "/api/v1/admin/finance/settlements", nil)
	req.Header.Set("Authorization", "Bearer "+service)
	h.ServeHTTP(httptest.NewRecorder(), req)
	if checks != 0 || calls != 2 {
		t.Fatalf("guest/service boundary checks=%d calls=%d", checks, calls)
	}
}

func TestConsumerIntrospectionDistinguishesRevocationFromDependencyOutage(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		status     int
		want       error
	}{
		{"current", `{"code":0}`, 200, nil}, {"revoked", `{"code":40105}`, 200, common.ErrSessionRevoked},
		{"db unavailable", `{"code":50001}`, 200, common.ErrDependencyUnavailable},
		{"bad response", `<html>error</html>`, 502, common.ErrDependencyUnavailable},
		{"missing code", `{}`, 200, common.ErrDependencyUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			s := &SecurityClient{AuthURL: "http://auth.internal:8081", Client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader(tc.body)), Header: make(http.Header)}, nil
			})}}
			err := s.ValidateAccess(context.Background(), "signed-consumer-token", &common.CustomClaims{UserType: "user"})
			if !errors.Is(err, tc.want) || calls != 1 {
				t.Fatalf("want=%v got=%v calls=%d", tc.want, err, calls)
			}
		})
	}
}

// The response is already streaming when authority changes. Cancellation must
// close the stream, never mix a JSON error envelope into the SSE protocol.
func TestEstablishedHumanStreamCancelsWithoutAppendingJSON(t *testing.T) {
	for _, mode := range []string{"ban", "dependency unavailable", "session revoked"} {
		t.Run(mode, func(t *testing.T) {
			var checks atomic.Int32
			options := AuthOptions{RecheckInterval: 10 * time.Millisecond, ValidateAccess: func(context.Context, string, *common.CustomClaims) error {
				if checks.Add(1) > 1 {
					if mode == "ban" {
						return common.ErrSessionRevoked
					}
					if mode == "dependency unavailable" {
						return common.ErrDependencyUnavailable
					}
				}
				return nil
			}}
			if mode == "session revoked" {
				var sessionChecks atomic.Int32
				options.SessionChecks = []func(context.Context, *common.CustomClaims) error{func(context.Context, *common.CustomClaims) error {
					if sessionChecks.Add(1) > 1 {
						return common.ErrSessionRevoked
					}
					return nil
				}}
			}
			h := AuthWithOptions("consumer-stream-test", nil, options)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, "data: started\n\n")
				w.(http.Flusher).Flush()
				select {
				case <-r.Context().Done():
				case <-time.After(time.Second):
					t.Error("stream was not canceled")
				}
			}))
			token, _ := common.GenAccessToken("consumer-stream-test", common.TokenInfo{UserId: 7, UserType: "user", Roles: []string{"customer"}, ClientID: "customer"}, 60)
			req := httptest.NewRequest("POST", "/api/v1/ai/sessions/1/messages", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Body.String() != "data: started\n\n" {
				t.Fatal("stream was polluted with a JSON error", w.Body.String())
			}
		})
	}
}

func TestStreamAuthorityTimeoutIsBounded(t *testing.T) {
	var checks atomic.Int32
	h := AuthWithOptions("consumer-stream-test", nil, AuthOptions{RecheckInterval: time.Millisecond, ValidateAccess: func(ctx context.Context, _ string, _ *common.CustomClaims) error {
		if checks.Add(1) == 1 {
			return nil
		}
		<-ctx.Done()
		return ctx.Err()
	}})(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
			t.Error("authority lookup had no deadline")
		}
	}))
	token, _ := common.GenAccessToken("consumer-stream-test", common.TokenInfo{UserId: 7, UserType: "user", Roles: []string{"customer"}, ClientID: "customer"}, 60)
	req := httptest.NewRequest("GET", "/api/v1/users/export", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	start := time.Now()
	h.ServeHTTP(httptest.NewRecorder(), req)
	if time.Since(start) > 2500*time.Millisecond {
		t.Fatal("authority timeout exceeded two-second bound")
	}
}

func TestShortRequestsStopMonitoringAndInternalStreamsSkipIt(t *testing.T) {
	var checks atomic.Int32
	h := AuthWithOptions("consumer-stream-test", nil, AuthOptions{RecheckInterval: 30 * time.Millisecond, ValidateAccess: func(context.Context, string, *common.CustomClaims) error { checks.Add(1); return nil }})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	token, _ := common.GenAccessToken("consumer-stream-test", common.TokenInfo{UserId: 7, UserType: "user", Roles: []string{"customer"}, ClientID: "customer"}, 60)
	req := httptest.NewRequest("GET", "/api/v1/users/profile", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	h.ServeHTTP(httptest.NewRecorder(), req)
	time.Sleep(60 * time.Millisecond)
	if checks.Load() != 1 {
		t.Fatal("finished request continued querying authority")
	}
	service, _ := common.SignServiceToken("consumer-stream-test", "order-service")
	req = httptest.NewRequest("POST", "/api/v1/admin/orders", nil)
	req.Header.Set("Authorization", "Bearer "+service)
	h.ServeHTTP(httptest.NewRecorder(), req)
	time.Sleep(60 * time.Millisecond)
	if checks.Load() != 1 {
		t.Fatal("internal service acquired human stream monitoring")
	}
}

func TestRealProxyClosesEstablishedSSEOnConsumerRevocation(t *testing.T) {
	var revoked atomic.Bool
	authority := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if revoked.Load() {
			_, _ = io.WriteString(w, `{"code":40105}`)
		} else {
			_, _ = io.WriteString(w, `{"code":0}`)
		}
	}))
	defer authority.Close()
	upstreamCanceled := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(upstreamCanceled)
	}))
	defer upstream.Close()
	security := &SecurityClient{AuthURL: authority.URL}
	gateway := httptest.NewServer(AuthWithOptions("consumer-proxy-test", nil, AuthOptions{ValidateAccess: security.ValidateAccess, RecheckInterval: 10 * time.Millisecond})(proxy.New([]config.Upstream{{Prefix: "/api/v1/ai", Target: strings.TrimPrefix(upstream.URL, "http://")}}, nil)))
	defer gateway.Close()
	token, _ := common.GenAccessToken("consumer-proxy-test", common.TokenInfo{UserId: 7, UserType: "user", Roles: []string{"customer"}, ClientID: "customer"}, 60)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", gateway.URL+"/api/v1/ai/stream", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	first := make([]byte, len("data: first\n\n"))
	if _, err = io.ReadFull(res.Body, first); err != nil {
		t.Fatal("SSE did not begin", err)
	}
	revoked.Store(true)
	tail, readErr := io.ReadAll(res.Body)
	if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) {
		t.Fatal("SSE ended for an unexpected reason", readErr)
	}
	if len(tail) != 0 || strings.Contains(string(first), "code") {
		t.Fatal("JSON error appended into an existing SSE stream")
	}
	select {
	case <-upstreamCanceled:
	case <-ctx.Done():
		t.Fatal("proxy left the upstream request running after revocation")
	}
	if ctx.Err() != nil {
		t.Fatal("stream only stopped at client timeout")
	}
}
