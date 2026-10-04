package weknora

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestModelDTONeverExposesCredentials(t *testing.T) {
	var m EngineModel
	json.Unmarshal([]byte(`{"id":"x","parameters":{"api_key":"private","base_url":"https://example.org/v1","extra_config":{"secret":"private"}}}`), &m)
	raw, _ := json.Marshal(m)
	if strings.Contains(string(raw), "private") {
		t.Fatal("secret escaped")
	}
}
func TestRerankRejectsPartialOrDuplicateResults(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		valid      bool
	}{{"ranked", `[{"index":1,"relevance_score":0.9},{"index":0,"relevance_score":0.1}]`, true}, {"duplicate", `[{"index":0,"relevance_score":1},{"index":0,"relevance_score":0}]`, false}, {"missing", `[{"index":0,"relevance_score":1}]`, false}, {"foreign", `[{"index":9,"relevance_score":1},{"index":0,"relevance_score":0}]`, false}} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "GET" {
					w.Write([]byte(`{"success":true,"data":{"type":"Rerank"}}`))
					return
				}
				r.ParseForm()
				if r.Form.Get("input") != "问题" || r.Form.Get("documents") == "" {
					t.Error("wrong debug wire format")
				}
				w.Write([]byte(`{"success":true,"data":{"ok":true,"raw_response":` + tc.body + `}}`))
			}))
			defer srv.Close()
			c, _ := New(srv.URL, "key", "e")
			s := &Service{Client: c}
			out, e := s.rerank(context.Background(), "rerank", "问题", []searchHit{{ID: "a", Content: "first"}, {ID: "b", Content: "second"}})
			if tc.valid {
				if e != nil || len(out) != 2 || out[0].ID != "b" || out[0].Score != .9 {
					t.Fatalf("lost rerank: %+v %v", out, e)
				}
			} else if e == nil {
				t.Fatal("accepted malformed ranking")
			}
		})
	}
}
func TestModelEndpointRestrictions(t *testing.T) {
	good := ModelInput{Name: "x", Type: "Embedding", Provider: "openai", BaseURL: "https://example.org/v1", Dimension: 512}
	if validateModel(good) != nil {
		t.Fatal("valid rejected")
	}
	for _, u := range []string{"http://example.org", "https://name:pass@example.org", "https://example.org:9000", "https://example.org/v1?key=secret", "file:///tmp/x"} {
		v := good
		v.BaseURL = u
		if validateModel(v) == nil {
			t.Fatal(u)
		}
	}
	for _, id := range []string{"../model", "x/y", "x?token=1", ""} {
		if validModelID(id) {
			t.Fatal(id)
		}
	}
}
