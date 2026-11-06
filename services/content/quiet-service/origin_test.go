package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestPublicOriginsFailClosed(t *testing.T) {
	for _, raw := range []string{"http://example.com", "https://*.example.com", "https://user@example.com", "https://example.com/", "https://example.com?x=1", "https://example.com#x", "https://example.com,", "https://example.com/path"} {
		if _, err := parseOrigins(raw); err == nil {
			t.Errorf("accepted invalid origin %q", raw)
		}
	}
	origins, err := parseOrigins("https://example.com, https://other.example.com")
	if err != nil {
		t.Fatal(err)
	}
	for _, origin := range []string{"", "https://example.com.attacker.test", "http://example.com", "https://example.com:444", "https://example.com/"} {
		if allowedOrigin(origin, origins) {
			t.Errorf("accepted unlisted origin %q", origin)
		}
	}
	if !allowedOrigin("https://example.com", origins) || !allowedOrigin("http://localhost:5186", origins) {
		t.Fatal("configured or development origin rejected")
	}
	for _, prefix := range []string{"quiet-*:", "other:", "quiet-park-prod", "quiet-park-prod:[x]:"} {
		if redisNamespace.MatchString(prefix) {
			t.Errorf("unsafe Redis namespace %q", prefix)
		}
	}
}

func TestConfiguredPublicOriginSessionAndWebSocket(t *testing.T) {
	a, err := newApp(memoryStorage(), "origin-test-secret")
	if err != nil {
		t.Fatal(err)
	}
	defer a.close()
	a.origins = []string{"https://example.com"}
	srv := httptest.NewServer(a.handler())
	defer srv.Close()
	b := newBrowser(t, srv.URL, false)
	if status, _ := b.call(t, "GET", "/quiet-api/session", nil, "", a.origins[0]); status != 200 {
		t.Fatal("configured public session rejected", status)
	}
	for _, origin := range []string{"https://example.com", "https://example.com.attacker.test"} {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		c, response, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/quiet-ws", &websocket.DialOptions{HTTPClient: b.client, HTTPHeader: http.Header{"Origin": {origin}}})
		if origin == a.origins[0] {
			if err != nil {
				t.Fatal("configured public WebSocket rejected", err)
			}
			readUntil(t, c, func(e event) bool { return e.Type == "welcome" })
			_ = c.CloseNow()
		} else if err == nil || response == nil || response.StatusCode != 403 {
			t.Fatal("unlisted WebSocket origin was not rejected")
		}
		cancel()
	}
}
