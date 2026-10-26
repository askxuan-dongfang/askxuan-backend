package modeling

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestModelingSecretAndTransportBoundaries(t *testing.T) {
	m := New(nil, strings.Repeat("s", 40), "")
	sealed, e := m.seal("fixture-model-key")
	if e != nil || strings.Contains(sealed, "fixture-model-key") {
		t.Fatal("secret storage invalid", e)
	}
	restored, e := m.open(sealed)
	if e != nil || restored != "fixture-model-key" {
		t.Fatal("secret restart failed", e)
	}
	other := New(nil, strings.Repeat("x", 40), "")
	if _, e = other.open(sealed); e == nil {
		t.Fatal("changed secret decrypted configuration")
	}
	for _, ip := range []string{"127.0.0.1", "169.254.169.254", "10.0.0.1", "::1", "fc00::1", "0.0.0.0"} {
		if publicIP(net.ParseIP(ip)) {
			t.Fatal("private endpoint accepted", ip)
		}
	}
	for _, u := range []string{"http://assets.meshy.ai/x", "https://127.0.0.1/model.glb", "https://assets.meshy.ai:8443/x", "https://user:pass@assets.meshy.ai/x", "https://evil.example/x"} {
		if _, e = downloadModel(context.Background(), u); e == nil {
			t.Fatal("unsafe URL accepted", u)
		}
	}
}
func TestProviderDoesNotEchoCredentialsOrRetryCreation(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("Authorization") != "Bearer fixture-private-key" {
			t.Error("missing credentials")
		}
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":"fixture-private-key"}`))
	}))
	defer s.Close()
	m := New(nil, strings.Repeat("s", 40), "")
	m.ProviderURL = s.URL
	m.Client = &http.Client{Timeout: time.Second}
	var result map[string]any
	e := m.provider(context.Background(), http.MethodPost, "/openapi/v1/multi-image-to-3d", "fixture-private-key", map[string]any{"image_urls": []string{"/photo"}}, &result)
	if e == nil || strings.Contains(e.Error(), "fixture-private-key") || calls != 1 {
		t.Fatal("provider failure leaked/retried", e, calls)
	}
	public, _ := json.Marshal(Config{Provider: "meshy", HasAPIKey: true})
	if strings.Contains(string(public), "fixture-private-key") {
		t.Fatal("public credential leak")
	}
}
