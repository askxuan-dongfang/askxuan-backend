package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/askxuan/common"
	"github.com/askxuan/temple-service/internal/svc"
)

func TestTempleStateWritesRequireLiveAuthorityBeforeParsing(t *testing.T) {
	for _, tc := range []struct {
		name, bearer, body string
		code               int
	}{
		{"unsigned forged headers", "", "not-json", 40101},
		{"empty bearer", "Bearer ", "not-json", 40101},
		{"missing fixed authority", "Bearer fixture", "not-json", 50205},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc := &svc.ServiceContext{}
			req := httptest.NewRequest("PUT", "/api/v1/admin/platform/temples/guess/state", strings.NewReader(tc.body))
			req.Header.Set("X-User-Id", "1")
			req.Header.Set("X-User-Roles", "platform_super")
			req.Header.Set("Authorization", tc.bearer)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			newTempleStateHandler(sc)(w, req)
			var response struct {
				Code int `json:"code"`
			}
			if json.Unmarshal(w.Body.Bytes(), &response) != nil || response.Code != tc.code {
				t.Fatalf("request reached state parsing/storage: %s", w.Body.String())
			}
		})
	}
}

func TestTempleStateAuthorityUsesCurrentRoleAndVersionForBothWriteEntrypoints(t *testing.T) {
	for _, path := range []string{"/api/v1/admin/platform/temples/T001/state", "/api/v1/admin/platform/temples/T001/status"} {
		for _, tc := range []struct {
			name, response string
			want           int
		}{
			{"super", `{"code":0,"data":{"userId":7,"roles":["platform_super"],"clientId":"platform-admin","authzScope":"platform"}}`, 0},
			{"delegated writer", `{"code":0,"data":{"userId":7,"roles":["fixture_operator"],"clientId":"platform-admin","authzScope":"platform","permissions":["platform.temples.write"]}}`, 0},
			{"read only", `{"code":0,"data":{"userId":7,"roles":["fixture_reader"],"clientId":"platform-admin","authzScope":"platform","permissions":["platform.temples.read"]}}`, 40304},
			{"consumer", `{"code":0,"data":{"userId":7,"roles":["customer"],"clientId":"customer"}}`, 40304},
			{"provider supplied permission", `{"code":0,"data":{"userId":7,"roles":["temple_admin"],"clientId":"platform-admin","authzScope":"platform","permissions":["platform.temples.write"]}}`, 40304},
			{"wrong scope", `{"code":0,"data":{"userId":7,"roles":["fixture_operator"],"clientId":"platform-admin","authzScope":"temple","permissions":["platform.temples.write"]}}`, 40304},
			{"revoked", `{"code":40105}`, 40105},
			{"invalid signature", `{"code":40102}`, 40102},
			{"authority DB failure", `{"code":50001}`, 50205},
			{"missing envelope code", `{}`, 50205},
			{"malformed authority response", `<html>failed</html>`, 50205},
		} {
			t.Run(path+" "+tc.name, func(t *testing.T) {
				var authorityCalls atomic.Int32
				authority := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					authorityCalls.Add(1)
					if r.Method != "GET" || r.URL.Path != "/api/v1/auth/access" || r.URL.RawQuery != "" || r.Header.Get("Authorization") != "Bearer original-signed-token" || r.Header.Get("X-User-Id") != "" || r.Header.Get("X-User-Roles") != "" {
						t.Error("untrusted target/identity forwarded into authority")
					}
					_, _ = w.Write([]byte(tc.response))
				}))
				defer authority.Close()
				sc := &svc.ServiceContext{}
				sc.Config.AuthServiceURL = authority.URL
				calls := 0
				protected := authorizeTempleState(sc, func(_ http.ResponseWriter, r *http.Request) {
					calls++
					if r.Header.Get("X-User-Id") != "7" || svc.UserIDFromCtx(r.Context()) != 7 || svc.TempleIDFromCtx(r.Context()) != 0 || r.Header.Get("X-Temple-Id") != "" {
						t.Error("forged actor/scope survived current-authority validation")
					}
				})
				req := httptest.NewRequest("PUT", path+"?private=not-forwarded", nil)
				req.Header.Set("Authorization", "Bearer original-signed-token")
				req.Header.Set("X-User-Id", "999")
				req.Header.Set("X-Temple-Id", "998")
				req.Header.Set("X-User-Roles", "platform_super")
				w := httptest.NewRecorder()
				protected(w, req)
				if authorityCalls.Load() != 1 {
					t.Fatal("authority was skipped")
				}
				if tc.want == 0 {
					if calls != 1 {
						t.Fatal("current authorised writer was denied", w.Body.String())
					}
					return
				}
				var response struct {
					Code int `json:"code"`
				}
				_ = json.Unmarshal(w.Body.Bytes(), &response)
				if calls != 0 || response.Code != tc.want {
					t.Fatalf("unauthorised request reached write: calls=%d response=%s", calls, w.Body.String())
				}
			})
		}
	}
}

func TestTempleStateAuthorityFailureAndRedirectNeverOpenWrite(t *testing.T) {
	var leaked atomic.Int32
	sink := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { leaked.Add(1) }))
	defer sink.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, sink.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()
	for _, target := range []string{"", "file:///private/secret", "http://user:password@localhost:1", redirect.URL} {
		sc := &svc.ServiceContext{}
		sc.Config.AuthServiceURL = target
		req := httptest.NewRequest("PUT", "/api/v1/admin/platform/temples/T001/state", nil)
		req.Header.Set("Authorization", "Bearer sensitive-fixture")
		if _, err := templeStateAuthority(sc, req); err != common.ErrDependencyUnavailable {
			t.Fatal("bad/unavailable authority opened access", err)
		}
	}
	if leaked.Load() != 0 {
		t.Fatal("fixed authority followed a redirect with a bearer")
	}
	slow := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer slow.Close()
	sc := &svc.ServiceContext{}
	sc.Config.AuthServiceURL = slow.URL
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	req := httptest.NewRequest("PUT", "/api/v1/admin/platform/temples/T001/state", nil).WithContext(ctx)
	req.Header.Set("Authorization", "Bearer fixture")
	if _, err := templeStateAuthority(sc, req); err != common.ErrDependencyUnavailable {
		t.Fatal("timed-out authority opened access", err)
	}
}
