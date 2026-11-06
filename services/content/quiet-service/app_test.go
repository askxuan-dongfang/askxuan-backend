package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type browser struct {
	client     *http.Client
	base, csrf string
}

func newBrowser(t *testing.T, base string, persistent bool) *browser {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	b := &browser{client: &http.Client{Jar: jar, Timeout: 5 * time.Second}, base: base}
	b.session(t, persistent)
	return b
}

func (b *browser) call(t *testing.T, method, path string, body any, csrf, origin string) (int, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, b.base+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", origin)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Quiet-CSRF", csrf)
	req.Header.Set("Idempotency-Key", "test-request-0001")
	response, err := b.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, data
}

func (b *browser) session(t *testing.T, persistent bool) {
	t.Helper()
	req, _ := http.NewRequest("GET", b.base+"/quiet-api/session", nil)
	response, err := b.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var data struct {
		CSRF       string `json:"csrfToken"`
		Persistent bool   `json:"persistent"`
	}
	if response.StatusCode != 200 || json.NewDecoder(response.Body).Decode(&data) != nil || len(data.CSRF) != 64 || data.Persistent != persistent {
		t.Fatal("guest session bootstrap failed", response.StatusCode)
	}
	for _, cookie := range response.Cookies() {
		if cookie.Name == guestCookie && (!cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || strings.HasPrefix(b.base, "https:") && !cookie.Secure) {
			t.Fatal("guest cookie lacks browser protections")
		}
	}
	b.csrf = data.CSRF
}

func (b *browser) list(t *testing.T) []practiceRecord {
	t.Helper()
	status, data := b.call(t, "GET", "/quiet-api/records", nil, "", "http://localhost:5186")
	if status != 200 {
		t.Fatalf("record list failed: %d %s", status, data)
	}
	var records []practiceRecord
	if err := json.Unmarshal(data, &records); err != nil {
		t.Fatal(err)
	}
	return records
}

func (b *browser) dial(t *testing.T) *websocket.Conn {
	t.Helper()
	parsed, _ := url.Parse(b.base)
	headers := http.Header{"Origin": []string{"http://localhost:5186"}}
	for _, cookie := range b.client.Jar.Cookies(parsed) {
		headers.Add("Cookie", cookie.String())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(b.base, "http")+"/quiet-ws", &websocket.DialOptions{HTTPHeader: headers})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestGuestRecordIsolationAndCSRF(t *testing.T) {
	service, err := newApp(memoryStorage(), "test-csrf-secret")
	if err != nil {
		t.Fatal(err)
	}
	defer service.close()
	srv := httptest.NewServer(service.handler())
	defer srv.Close()
	a, b := newBrowser(t, srv.URL, false), newBrowser(t, srv.URL, false)
	record := practiceRecord{Date: time.Now().UTC().Truncate(time.Millisecond).Format(time.RFC3339Nano), Seconds: 30, Hits: 3, Beads: 108}
	for _, test := range []struct{ csrf, origin string }{{"", "http://localhost:5186"}, {a.csrf, "https://evil.example"}, {b.csrf, "http://localhost:5186"}} {
		if status, _ := a.call(t, "POST", "/quiet-api/records", record, test.csrf, test.origin); status != 403 {
			t.Fatal("unsafe save accepted", status)
		}
	}
	for i := 0; i < 2; i++ {
		if status, data := a.call(t, "POST", "/quiet-api/records", record, a.csrf, "http://localhost:5186"); status != 200 {
			t.Fatalf("save failed: %d %s", status, data)
		}
	}
	if len(a.list(t)) != 1 || len(b.list(t)) != 0 {
		t.Fatal("record isolation or idempotency failed")
	}
	if status, _ := b.call(t, "DELETE", "/quiet-api/records", nil, b.csrf, "http://localhost:5186"); status != 200 || len(a.list(t)) != 1 {
		t.Fatal("another guest deleted visitor records")
	}
	if status, _ := a.call(t, "POST", "/quiet-api/records", map[string]any{"date": record.Date, "guestID": "other", "seconds": 1, "hits": 1, "beads": 1}, a.csrf, "http://localhost:5186"); status != 400 {
		t.Fatal("client supplied guest identity accepted")
	}
	if status, _ := a.call(t, "DELETE", "/quiet-api/records", nil, a.csrf, "http://localhost:5186"); status != 200 || len(a.list(t)) != 0 {
		t.Fatal("own deletion failed")
	}
}

func TestAutomaticRoomAllocation(t *testing.T) {
	service, err := newApp(memoryStorage(), "test-csrf-secret")
	if err != nil {
		t.Fatal(err)
	}
	defer service.close()
	srv := httptest.NewServer(service.handler())
	defer srv.Close()
	b := newBrowser(t, srv.URL, false)
	counts := map[string]int{}
	for i := 0; i < 21; i++ {
		c := b.dial(t)
		defer c.CloseNow()
		welcome := readUntil(t, c, func(e event) bool { return e.Type == "welcome" })
		if welcome.RoomID == "" {
			t.Fatal("missing room identifier")
		}
		counts[welcome.RoomID]++
	}
	if len(counts) != 2 {
		t.Fatal("21 visitors were not allocated into two rooms", counts)
	}
	for _, count := range counts {
		if count != 20 && count != 1 {
			t.Fatal("room allocation exceeded boundary", count)
		}
	}
}

func waitUntil(t *testing.T, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !check() {
		if time.Now().After(deadline) {
			t.Fatal("expected storage state did not appear")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func guestHashFor(t *testing.T, b *browser) string {
	t.Helper()
	parsed, _ := url.Parse(b.base)
	for _, cookie := range b.client.Jar.Cookies(parsed) {
		if cookie.Name == guestCookie {
			return tokenHash(cookie.Value)
		}
	}
	t.Fatal("missing guest cookie")
	return ""
}
