package websearch

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"testing"
)

func TestSessionBudgetAndSourceBoundReader(t *testing.T) {
	s := (Config{Provider: "bocha", APIKey: "secret", Options: Options{MaxSearches: 1, ReadPages: true}}).NewSession()
	searches, reads := 0, 0
	s.searchClient = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		searches++
		return response(`{"code":200,"data":{"webPages":{"value":[{"name":"古籍","url":"https://example.org/book","summary":"摘要"}]}}}`), nil
	})}
	s.pageClient = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		reads++
		if r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Fatal("credentials leaked to webpage")
		}
		resp := response(`<html><head><script>evil()</script></head><body><nav>导航</nav><h1>古籍原文</h1><p>第一章</p></body></html>`)
		resp.Header = http.Header{"Content-Type": []string{"text/html; charset=utf-8"}}
		return resp, nil
	})}
	if _, err := s.Read(context.Background(), "https://example.org/book"); err == nil || reads != 0 {
		t.Fatal("read before search")
	}
	if _, err := s.Search(context.Background(), "古籍"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Search(context.Background(), "古籍"); err == nil || searches != 1 {
		t.Fatal("unbounded search")
	}
	if _, err := s.Read(context.Background(), "https://unlisted.org/book"); err == nil || reads != 0 {
		t.Fatal("arbitrary URL accepted")
	}
	raw, err := s.Read(context.Background(), "https://example.org/book")
	var page struct{ Text string }
	_ = json.Unmarshal([]byte(raw), &page)
	if err != nil || !strings.Contains(page.Text, "第一章") || strings.Contains(page.Text, "evil()") || strings.Contains(page.Text, "导航") {
		t.Fatalf("%s %v", raw, err)
	}
	if _, err = s.Read(context.Background(), "https://example.org/book"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Read(context.Background(), "https://example.org/book"); err == nil || reads != 2 {
		t.Fatal("unbounded page reads")
	}
	other := (Config{Provider: "bocha", APIKey: "secret", Options: Options{ReadPages: true}}).NewSession()
	if _, err = other.Read(context.Background(), "https://example.org/book"); err == nil {
		t.Fatal("cross-session URL reuse")
	}
}
func TestPageNetworkAndContentBounds(t *testing.T) {
	for _, ip := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "100.64.0.1", "::1", "::ffff:127.0.0.1", "fc00::1", "2001:db8::1"} {
		if publicPageIP(netip.MustParseAddr(ip)) {
			t.Fatal("allowed private/reserved IP", ip)
		}
	}
	if !publicPageIP(netip.MustParseAddr("8.8.8.8")) {
		t.Fatal("blocked public IP")
	}
	s := (Config{Options: Options{ReadPages: true}}).NewSession()
	s.urls["http://127.0.0.1/"] = true
	if _, err := s.Read(context.Background(), "http://127.0.0.1/"); err == nil {
		t.Fatal("reader reached loopback")
	}
	for _, tc := range []struct {
		kind, body string
		wantError  bool
	}{{"application/pdf", "pdf", true}, {"text/plain", strings.Repeat("a", (2<<20)+1), true}, {"text/plain", strings.Repeat("文", 13000), false}} {
		s := (Config{Options: Options{ReadPages: true}}).NewSession()
		s.urls["https://example.org/"] = true
		s.pageClient = &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{tc.kind}}, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
		})}
		raw, err := s.Read(context.Background(), "https://example.org/")
		if (err != nil) != tc.wantError {
			t.Fatal("content limit ignored", err)
		}
		if err == nil {
			var out struct {
				Text      string
				Truncated bool
			}
			json.Unmarshal([]byte(raw), &out)
			if !out.Truncated || len([]rune(out.Text)) != 12000 {
				t.Fatal("truncation not reported")
			}
		}
	}
}
