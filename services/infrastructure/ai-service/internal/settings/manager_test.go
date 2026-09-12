package settings

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/askxuan/ai-service/internal/config"
	"github.com/askxuan/ai-service/internal/provider"
)

func fixture(t *testing.T) (*Manager, string, string) {
	t.Helper()
	dir := t.TempDir()
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	initial := config.AIConf{Provider: "openai_compatible", BaseURL: "https://api.deepseek.com", APIKey: "fixture-old-key", Model: "deepseek-flash", VisionModel: "deepseek-flash", MaxOutputTokens: 2048, ReasoningEffort: "low"}
	m, err := New(initial, dir, key)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer invalid-fixture-key" {
			http.Error(w, "never echo upstream bodies", 401)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"deepseek-flash"},{"id":"deepseek-v4-pro"}]}`))
	}))
	t.Cleanup(server.Close)
	m.build = func(r record) (*Snapshot, error) {
		p := provider.NewOpenAICompatible(server.URL, r.APIKey, r.DefaultModel, r.VisionModel)
		c := m.original
		c.Model = r.DefaultModel
		c.APIKey = r.APIKey
		c.MaxOutputTokens = r.MaxOutputTokens
		return &Snapshot{Config: c, Provider: p, Models: provider.NewCatalogWithAllowed(p, r.EnabledModels)}, nil
	}
	return m, dir, key
}
func update(m *Manager) Update {
	p := m.Public()
	return Update{Values: p.Values, Revision: p.Revision}
}

func TestSaveEncryptedRedactedAndSurvivesRestart(t *testing.T) {
	m, dir, key := fixture(t)
	before := m.Snapshot()
	req := update(m)
	req.DefaultModel = "deepseek-v4-pro"
	req.APIKey = "fixture-new-key"
	res, err := m.Save(context.Background(), req, "42")
	if err != nil {
		t.Fatal(err)
	}
	if res.Revision != 1 || !res.HasAPIKey || res.Source != "platform" || res.History[0].Actor != "42" {
		t.Fatalf("invalid public settings: %+v", res)
	}
	if before.Provider.Model() != "deepseek-flash" || m.Snapshot().Provider.Model() != "deepseek-v4-pro" {
		t.Fatal("request snapshot was mutated")
	}
	for _, data := range [][]byte{mustJSON(t, res), mustRead(t, filepath.Join(dir, "settings.enc"))} {
		if strings.Contains(string(data), "fixture-new-key") {
			t.Fatal("secret escaped")
		}
	}
	info, _ := os.Stat(filepath.Join(dir, "settings.enc"))
	if info.Mode().Perm() != 0600 {
		t.Fatal("secret file permissions")
	}
	restarted, err := New(m.original, dir, key)
	if err != nil {
		t.Fatal(err)
	}
	if restarted.Snapshot().Config.APIKey != "fixture-new-key" || restarted.Public().Revision != 1 {
		t.Fatal("persisted settings not restored")
	}
	second := update(m)
	second.MaxOutputTokens = 4096
	if _, err = m.Save(context.Background(), second, "43"); err != nil {
		t.Fatal(err)
	}
	if len(mustRead(t, filepath.Join(dir, "previous.enc"))) == 0 {
		t.Fatal("missing encrypted previous revision")
	}
}
func TestValidationAndFailedSavesKeepCurrentRuntime(t *testing.T) {
	m, _, _ := fixture(t)
	for _, change := range []func(*Update){
		func(r *Update) { r.APIKey = "invalid-fixture-key" },
		func(r *Update) { r.DefaultModel = "nonexistent" },
		func(r *Update) { r.VisionModel = "deepseek-v4-pro" },
		func(r *Update) { r.EnabledModels = []string{"deepseek-v4-pro"} },
		func(r *Update) { r.Provider = "openai_compatible"; r.BaseURL = "https://example.com/v1" },
		func(r *Update) { r.BaseURL = "https://127.0.0.1"; r.APIKey = "new-fixture-key" },
	} {
		req := update(m)
		change(&req)
		if _, err := m.Save(context.Background(), req, "1"); err == nil {
			t.Fatal("invalid settings accepted")
		}
		if m.Public().Revision != 0 || m.Snapshot().Config.APIKey != "fixture-old-key" {
			t.Fatal("failed save changed runtime")
		}
	}
	m.store.dir = filepath.Join(t.TempDir(), "absent")
	if _, err := m.Save(context.Background(), update(m), "1"); err == nil {
		t.Fatal("persistence failure accepted")
	}
	if m.Public().Revision != 0 {
		t.Fatal("failed disk write published runtime")
	}
}
func TestConcurrentSaveAndEnabledCatalog(t *testing.T) {
	m, _, _ := fixture(t)
	req := update(m)
	req.EnabledModels = []string{"deepseek-flash"}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := m.Save(context.Background(), req, "1"); results <- err }()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, ErrConflict) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatalf("saves succeeded: %d", success)
	}
	list, err := m.Snapshot().Models.List(context.Background())
	if err != nil || len(list.List) != 1 {
		t.Fatalf("allowlist not enforced: %+v %v", list, err)
	}
	if _, err = m.Snapshot().Models.Select(context.Background(), "deepseek-v4-pro", false); !errors.Is(err, provider.ErrModelUnavailable) {
		t.Fatal("hidden model accepted")
	}
}
func TestConnectionProbeDoesNotPersist(t *testing.T) {
	m, dir, _ := fixture(t)
	req := update(m)
	req.APIKey = "fixture-draft-key"
	list, err := m.Test(context.Background(), req)
	if err != nil || len(list.List) != 2 {
		t.Fatal(err)
	}
	if m.Public().Revision != 0 || m.Snapshot().Config.APIKey != "fixture-old-key" {
		t.Fatal("probe changed runtime")
	}
	if _, err = os.Stat(filepath.Join(dir, "settings.enc")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("probe persisted credentials")
	}
}
func TestStorageTamperingFailsClosed(t *testing.T) {
	m, dir, key := fixture(t)
	if _, err := m.Save(context.Background(), update(m), "1"); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "settings.enc")
	raw := mustRead(t, file)
	raw[len(raw)-1] ^= 1
	_ = os.WriteFile(file, raw, 0600)
	if _, err := New(m.original, dir, key); err == nil {
		t.Fatal("corrupt config silently fell back to environment")
	}
}
func TestEndpointBoundariesAndRedirects(t *testing.T) {
	for _, raw := range []string{"http://api.deepseek.com", "https://user:pass@example.com", "https://example.com?secret=x", "https://example.com:8443", "https://127.0.0.1", "https://[::1]", "https://169.254.169.254/latest", "https://example.com/../private"} {
		if _, err := normalizeURL(raw); err == nil {
			t.Fatal("accepted " + raw)
		}
	}
	for _, raw := range []string{"10.0.0.1", "172.16.1.1", "192.168.0.1", "169.254.169.254", "100.100.100.200", "::ffff:127.0.0.1", "fd00::1", "64:ff9b::a00:1"} {
		if publicIP(netip.MustParseAddr(raw)) {
			t.Fatal("allowed " + raw)
		}
	}
	if normalized, err := normalizeURL("https://API.DEEPSEEK.COM/v1/"); err != nil || normalized != "https://api.deepseek.com/v1" {
		t.Fatal(normalized, err)
	}
	c := secureClient()
	calls := 0
	c.Transport = roundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 302, Header: http.Header{"Location": []string{"https://example.org/steal"}}, Body: http.NoBody, Request: r}, nil
	})
	if _, err := c.Get("https://api.deepseek.com/models"); err == nil {
		t.Fatal("redirect accepted")
	}
	if calls != 1 {
		t.Fatal("credential request followed redirect")
	}
	c = secureClient()
	u, _ := url.Parse("https://127.0.0.1/models")
	_, err := c.Do(&http.Request{Method: "GET", URL: u, Header: make(http.Header)})
	if err == nil {
		t.Fatal("private dial allowed")
	}
}

type roundTrip func(*http.Request) (*http.Response, error)

func (f roundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, e := json.Marshal(v)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, e := os.ReadFile(p)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
