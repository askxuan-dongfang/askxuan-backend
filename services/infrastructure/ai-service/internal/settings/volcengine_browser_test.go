package settings

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
)

// Opt-in local browser harness: real settings manager and encrypted store, fake
// upstream only. No database, production credentials or external network calls.
func TestVolcengineBrowserFixture(t *testing.T) {
	if os.Getenv("ASKXUAN_VOLCENGINE_BROWSER_TEST") != "1" {
		t.Skip("opt-in Ego Lite browser fixture")
	}
	m, _, _ := fixture(t)
	var mu sync.Mutex
	var events []map[string]any
	recordEvent := func(event map[string]any) { mu.Lock(); events = append(events, event); mu.Unlock() }
	arkTransport(t, m, func(r *http.Request) (*http.Response, error) {
		if r.Method == "GET" && r.URL.Path == "/models" {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"data":[{"id":"deepseek-flash"},{"id":"deepseek-v4-pro"}]}`)), Header: make(http.Header)}, nil
		}
		var body struct {
			Model    string          `json:"model"`
			Messages json.RawMessage `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			return nil, err
		}
		status := 200
		if r.Header.Get("Authorization") != "Bearer ark-browser-fixture" {
			status = 401
		}
		if strings.Contains(body.Model, "missing") {
			status = 404
		}
		recordEvent(map[string]any{"kind": "upstream", "method": r.Method, "path": r.URL.Path, "model": body.Model, "status": status, "image": strings.Contains(string(body.Messages), "image_url")})
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(`{"model":"doubao-fixture","choices":[{"message":{"content":"OK"}}]}`)), Header: make(http.Header)}, nil
	})
	done := make(chan struct{})
	var stop sync.Once
	mux := http.NewServeMux()
	mux.HandleFunc("/_fixture/stop", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(405)
			return
		}
		stop.Do(func() { close(done) })
		fmt.Fprint(w, "stopping")
	})
	mux.HandleFunc("/_fixture/events", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		_ = json.NewEncoder(w).Encode(events)
	})
	mux.HandleFunc("/api/v1/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		var data any
		var err error
		switch r.URL.Path {
		case "/api/v1/ai/admin/provider", "/api/v1/ai/admin/provider/test":
			if r.Method == "GET" {
				data = m.Public()
				break
			}
			var input Update
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				w.WriteHeader(400)
				return
			}
			if r.Method == "PUT" {
				data, err = m.Save(r.Context(), input, "browser-fixture")
			} else {
				data, err = m.Test(r.Context(), input)
			}
			recordEvent(map[string]any{"kind": "settings", "method": r.Method, "path": r.URL.Path, "provider": input.Provider, "defaultModel": input.DefaultModel, "enabledModels": input.EnabledModels, "ok": err == nil, "revision": m.Public().Revision})
		case "/api/v1/ai/models":
			data, err = m.Snapshot().Models.List(r.Context())
		default:
			data = map[string]any{"list": []any{}, "total": 0}
		}
		if err != nil {
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 40001, "message": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 0, "data": data})
	})
	listener, err := net.Listen("tcp", "127.0.0.1:18546")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: mux}
	t.Cleanup(func() { _ = server.Close() })
	go func() { _ = server.Serve(listener) }()
	t.Log("local browser fixture ready on 127.0.0.1:18546; POST /_fixture/stop to finish")
	<-done
}
