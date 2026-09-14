package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/askxuan/common"
)

// Exercise the real gateway boundary before the community service's viewer check.
func TestPublicCommunityPreservesAuthenticatedViewer(t *testing.T) {
	const secret = "community-gateway-test"
	token, err := common.GenAccessToken(secret, common.TokenInfo{UserId: 42, UserType: "user", Roles: []string{"customer"}, ClientID: "customer"}, 60)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/v1/community/feed", "/api/v1/community/feed?following=true", "/api/v1/community/feed?type=article", "/api/v1/community/feed?type=video", "/api/v1/community/posts/P1"} {
		t.Run(path, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, path, nil)
			r.Header.Set("Authorization", "Bearer "+token)
			called := false
			h := Auth(secret, []string{"/api/v1/community/feed", "/api/v1/community/posts"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if r.Header.Get(HeaderUserID) != "42" || r.Header.Get(HeaderUserType) != "user" || r.Header.Get(HeaderRoles) != "customer" || r.Header.Get(HeaderClientID) != "customer" {
					t.Errorf("verified viewer was not forwarded: %v", r.Header)
				}
			}))
			h.ServeHTTP(httptest.NewRecorder(), r)
			if !called {
				t.Fatal("valid access token rejected")
			}
		})
	}
}

func TestPublicAuthTrustBoundary(t *testing.T) {
	const secret = "community-gateway-test"
	valid, _ := common.GenAccessToken(secret, common.TokenInfo{UserId: 42, UserType: "user"}, 60)
	expired, _ := common.GenAccessToken(secret, common.TokenInfo{UserId: 42, UserType: "user"}, -60)
	refresh, _ := common.GenRefreshToken(secret, common.TokenInfo{UserId: 42, UserType: "user"}, 60)
	headers := []string{HeaderUserID, HeaderUserMobile, HeaderUserType, HeaderRoles, HeaderClientID, HeaderTempleID, HeaderTempleCode, HeaderMasterID}
	for _, tc := range []struct {
		name, method, path, token, viewer string
		allowed                           bool
	}{
		{"guest feed", "GET", "/api/v1/community/feed", "", "", true},
		{"guest detail", "GET", "/api/v1/community/posts/P1", "", "", true},
		{"guest marketing", "GET", "/api/v1/marketing/activities/1", "", "", true},
		{"valid viewer", "GET", "/api/v1/community/feed?following=true", valid, "42", true},
		{"invalid token", "GET", "/api/v1/community/feed", "invalid", "", false},
		{"expired token", "GET", "/api/v1/community/feed", expired, "", false},
		{"refresh token", "GET", "/api/v1/community/feed", refresh, "", false},
		{"guest like", "POST", "/api/v1/community/posts/P1/like", "", "", false},
		{"guest following", "GET", "/api/v1/community/masters/following", "", "", false},
		{"similar prefix", "GET", "/api/v1/community/feed-private", "", "", false},
		{"login ignores stale bearer", "POST", "/api/v1/auth/login", "invalid", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(tc.method, tc.path, nil)
			for _, h := range headers {
				r.Header.Set(h, "spoofed")
			}
			if tc.token != "" {
				r.Header.Set("Authorization", "Bearer "+tc.token)
			}
			called := false
			h := Auth(secret, []string{"/api/v1/community/feed", "/api/v1/community/posts", "/api/v1/auth/login"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				if r.Header.Get(HeaderUserID) != tc.viewer {
					t.Errorf("viewer=%q, want %q", r.Header.Get(HeaderUserID), tc.viewer)
				}
				for _, h := range headers {
					if r.Header.Get(h) == "spoofed" {
						t.Errorf("untrusted identity header forwarded: %s", h)
					}
				}
			}))
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, r)
			if called != tc.allowed {
				t.Fatalf("reached downstream=%v, want %v", called, tc.allowed)
			}
			if !tc.allowed {
				var body common.Body
				if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil || body.Code == 0 {
					t.Fatalf("expected auth rejection: %s", rr.Body.String())
				}
			}
		})
	}
}
