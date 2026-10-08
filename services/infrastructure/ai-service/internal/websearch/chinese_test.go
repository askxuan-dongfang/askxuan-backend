package websearch

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

func response(body string) *http.Response {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}
}
func TestChineseProvidersAndCredentialIsolation(t *testing.T) {
	for _, p := range []string{"bocha", "tencent"} {
		t.Run(p, func(t *testing.T) {
			c := Config{Provider: p, APIKey: "primary-secret", Options: Options{Freshness: "week", IncludeDomains: "example.org", ExcludeDomains: "bad.example.org", MaxResults: 2}}
			client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				if r.Header.Get("Authorization") != "Bearer primary-secret" {
					t.Fatal("wrong credential")
				}
				var body map[string]any
				json.NewDecoder(r.Body).Decode(&body)
				if p == "bocha" {
					if r.URL.String() != "https://api.bochaai.com/v1/web-search" || body["query"] != "中文测试" || body["freshness"] != "oneWeek" || body["include"] != "example.org" {
						t.Fatalf("bad request %v", body)
					}
					return response(`{"code":200,"data":{"webPages":{"value":[{"name":"古籍","url":"https://example.org/a","summary":"中文摘要","datePublished":"2026-10-08"},{"url":"https://bad.example.org/a"},{"url":"https://other.org/a"}]}}}`), nil
				}
				if r.URL.String() != "https://api.wsa.cloud.tencent.com/SearchPro" || body["Query"] != "中文测试" || body["Site"] != "example.org" || body["FromTime"] == nil || body["Cnt"] != nil {
					t.Fatalf("bad Tencent request %v", body)
				}
				row, _ := json.Marshal(map[string]string{"title": "古籍", "url": "https://example.org/a", "passage": "中文摘要", "date": "2026-10-08"})
				raw, _ := json.Marshal(map[string]any{"Response": map[string]any{"Pages": []string{string(row)}}})
				return response(string(raw)), nil
			})}
			raw, err := c.search(context.Background(), "中文测试", client)
			if err != nil {
				t.Fatal(err)
			}
			var result Result
			json.Unmarshal([]byte(raw), &result)
			if result.Provider != p || len(result.Sources) != 1 || result.Sources[0].Snippet != "中文摘要" || result.Sources[0].Published != "2026-10-08" {
				t.Fatal(raw)
			}
		})
	}
}
func TestFallbackBoundedAndReported(t *testing.T) {
	c := Config{Provider: "bocha", APIKey: "primary", FallbackProvider: "tencent", FallbackAPIKey: "backup"}
	calls := 0
	client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return response(`{"code":403,"message":"primary"}`), nil
		}
		if r.URL.Host != "api.wsa.cloud.tencent.com" || r.Header.Get("Authorization") != "Bearer backup" {
			t.Fatal("cross-provider key leak")
		}
		return response(`{"Response":{"Pages":[]}}`), nil
	})}
	raw, err := c.searchWithFallback(context.Background(), "中文", client)
	if err != nil || calls != 2 || !strings.Contains(raw, `"fallbackFrom":"bocha"`) || !strings.Contains(raw, `"provider":"tencent"`) {
		t.Fatalf("%s %v %d", raw, err, calls)
	}
	calls = 0
	c.Provider = "tencent"
	c.FallbackProvider = "bocha"
	client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		return response(`{"Response":{"Pages":[]}}`), nil
	})
	if _, err = c.searchWithFallback(context.Background(), "中文", client); err != nil || calls != 1 {
		t.Fatal("empty search triggered paid fallback")
	}
	ctx, cancel := context.WithCancel(context.Background())
	calls = 0
	client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) { calls++; cancel(); return nil, errors.New("secret") })
	if _, err = c.searchWithFallback(ctx, "中文", client); err == nil || calls != 1 {
		t.Fatal("cancellation triggered fallback")
	}
}
func TestProviderApplicationErrorsAndSettings(t *testing.T) {
	for _, tc := range []struct{ p, body string }{{"bocha", `{"code":403,"message":"secret"}`}, {"bocha", `{"code":200}`}, {"tencent", `{"Response":{"Error":{"Message":"secret"}}}`}, {"tencent", `{"Response":{"Pages":["broken"]}}`}, {"tencent", `{"Response":{"Msg":"blocked","Pages":[]}}`}} {
		c := Config{Provider: tc.p, APIKey: "secret"}
		client := &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) { return response(tc.body), nil })}
		if _, err := c.search(context.Background(), "query", client); err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatal("provider error accepted/leaked")
		}
	}
	for _, o := range []Options{{IncludeDomains: "https://example.org"}, {ExcludeDomains: "localhost"}, {IncludeDomains: "127.0.0.1"}, {Freshness: "forever"}, {MaxResults: 11}, {Language: "unknown"}} {
		if o.Validate() == nil {
			t.Fatalf("accepted %v", o)
		}
	}
}
func TestBraveLanguageAndDomainFiltering(t *testing.T) {
	c := Config{Provider: "brave", APIKey: "secret", Options: Options{Language: "zh-hans", Country: "CN", Freshness: "day", IncludeDomains: "example.org"}}
	client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		q := r.URL.Query()
		if q.Get("search_lang") != "zh-hans" || q.Get("country") != "CN" || q.Get("freshness") != "pd" {
			t.Fatal(q)
		}
		return response(`{"web":{"results":[{"url":"https://notexample.org/a"},{"url":"https://book.example.org/a"}]}}`), nil
	})}
	raw, err := c.search(context.Background(), "中文", client)
	if err != nil || strings.Contains(raw, "notexample.org") || !strings.Contains(raw, "book.example.org") {
		t.Fatalf("%s %v", raw, err)
	}
}

func TestConfigJSONRedactsBothCredentials(t *testing.T) {
	c := Config{Provider: "bocha", APIKey: "secret-primary", FallbackProvider: "tencent", FallbackAPIKey: "secret-fallback"}
	raw, err := json.Marshal(c)
	if err != nil || strings.Contains(string(raw), "secret-") {
		t.Fatalf("configuration leaked: %s %v", raw, err)
	}
}
