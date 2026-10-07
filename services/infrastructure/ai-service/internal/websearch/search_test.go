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

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestProvidersBoundQueryAndSources(t *testing.T) {
	for _, p := range []string{"tavily", "brave"} {
		t.Run(p, func(t *testing.T) {
			c := Config{Provider: p, APIKey: "secret"}
			calls := 0
			client := &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				if p == "tavily" {
					if r.URL.String() != "https://api.tavily.com/search" || r.Header.Get("Authorization") != "Bearer secret" {
						t.Fatal("wrong endpoint/auth")
					}
					var body map[string]any
					json.NewDecoder(r.Body).Decode(&body)
					if body["query"] != "公共资料" || body["max_results"] != float64(5) || body["include_raw_content"] != false || body["search_depth"] != "basic" {
						t.Fatalf("unbounded request: %v", body)
					}
				} else if r.URL.Host != "api.search.brave.com" || r.URL.Query().Get("q") != "公共资料" || r.Header.Get("X-Subscription-Token") != "secret" {
					t.Fatal("wrong brave request")
				}
				rows := `[{"title":"原文","url":"https://example.org/book","content":"摘要","description":"摘要"},{"url":"javascript:alert(1)"},{"url":"http://127.0.0.1/a"},{"url":"https://example.org/book"}]`
				body := `{"results":` + rows + `}`
				if p == "brave" {
					body = `{"web":` + body + `}`
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
			})}
			raw, e := c.search(context.Background(), "公共资料", client)
			if e != nil {
				t.Fatal(e)
			}
			var result Result
			if json.Unmarshal([]byte(raw), &result) != nil || len(result.Sources) != 1 || result.Sources[0].Snippet != "摘要" || result.RetrievedAt == "" || strings.Contains(raw, "secret") {
				t.Fatalf("bad result: %s", raw)
			}
			for _, q := range []string{"", strings.Repeat("字", 401)} {
				if _, e = c.search(context.Background(), q, client); e == nil {
					t.Fatal("invalid query accepted")
				}
			}
			if calls != 1 {
				t.Fatal("invalid queries sent")
			}
		})
	}
}
func TestProviderFailuresNeverLeakBodiesOrKeys(t *testing.T) {
	for _, status := range []int{401, 402, 429, 500} {
		client := &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader("secret"))}, nil
		})}
		_, e := (Config{Provider: "tavily", APIKey: "secret"}).search(context.Background(), "test", client)
		if e == nil || strings.Contains(e.Error(), "secret") {
			t.Fatal("unsafe error")
		}
	}
	client := &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("secret") })}
	_, e := (Config{Provider: "brave", APIKey: "secret"}).search(context.Background(), "test", client)
	if e == nil || strings.Contains(e.Error(), "secret") {
		t.Fatal("transport error leaked")
	}
	if _, e = (Config{}).search(context.Background(), "test", client); e == nil {
		t.Fatal("unconfigured search allowed")
	}
}
func TestSearchBoundsResponse(t *testing.T) {
	for _, body := range []string{strings.Repeat("a", (1<<20)+1), "not-json"} {
		client := &http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
		})}
		if _, e := (Config{Provider: "tavily", APIKey: "secret"}).search(context.Background(), "test", client); e == nil {
			t.Fatal("invalid response accepted")
		}
	}
}
