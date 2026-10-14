package settings

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/askxuan/ai-service/internal/provider"
)

func arkUpdate(m *Manager) Update {
	r := update(m)
	r.Provider = "volcengine"
	r.BaseURL = provider.VolcengineBaseURL
	r.APIKey = "ark-fixture-key"
	r.DefaultModel = "ep-fixture-text"
	r.VisionModel = "ep-fixture-vision"
	r.EnabledModels = []string{"ep-fixture-text", "ep-fixture-vision", "doubao-fixture"}
	return r
}

func arkTransport(t *testing.T, m *Manager, transport roundTrip) {
	t.Helper()
	m.build = func(r record) (*Snapshot, error) {
		snap, err := m.buildSnapshot(r)
		if err == nil {
			snap.Provider.(*provider.OpenAICompatible).SetHTTPClient(&http.Client{Transport: transport})
		}
		return snap, err
	}
}

func TestVolcengineProbeSaveRestartAndSnapshot(t *testing.T) {
	m, dir, key := fixture(t)
	before := m.Snapshot()
	var mu sync.Mutex
	seen := map[string]int{}
	arkTransport(t, m, func(r *http.Request) (*http.Response, error) {
		if r.Method != "POST" || r.URL.String() != provider.VolcengineBaseURL+"/chat/completions" || r.Header.Get("Authorization") != "Bearer ark-fixture-key" {
			t.Errorf("unexpected probe request: %s %s", r.Method, r.URL)
		}
		var body struct {
			Model     string `json:"model"`
			MaxTokens int    `json:"max_tokens"`
			Messages  []struct {
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
			Thinking struct {
				Type string `json:"type"`
			} `json:"thinking"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.MaxTokens != 64 || body.Thinking.Type != "disabled" || len(body.Messages) != 1 {
			t.Errorf("unbounded or invalid probe: %+v", body)
		}
		if len(body.Messages) == 1 && strings.Contains(string(body.Messages[0].Content), "image_url") != (body.Model == "ep-fixture-vision") {
			t.Error("vision route not probed with an image")
		}
		mu.Lock()
		seen[body.Model]++
		mu.Unlock()
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"OK"}}]}`)), Header: make(http.Header)}, nil
	})
	req := arkUpdate(m)
	list, err := m.Test(context.Background(), req)
	if err != nil || len(list.List) != 3 {
		t.Fatalf("probe: %+v %v", list, err)
	}
	if m.Snapshot() != before || m.Public().Revision != 0 {
		t.Fatal("test published a draft")
	}
	if _, err = os.Stat(filepath.Join(dir, "settings.enc")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("test persisted a draft")
	}
	pub, err := m.Save(context.Background(), req, "42")
	if err != nil || pub.Provider != "volcengine" || pub.Revision != 1 || !pub.HasAPIKey {
		t.Fatalf("save: %+v %v", pub, err)
	}
	for _, id := range req.EnabledModels {
		if seen[id] != 2 {
			t.Errorf("%s was not probed in test and save: %d", id, seen[id])
		}
	}
	if before.Provider.Model() != "deepseek-flash" || m.Snapshot().Provider.Name() != "volcengine" || m.Snapshot().Config.Provider != "volcengine" {
		t.Fatal("runtime snapshot not switched safely")
	}
	if m.Snapshot().Config.DeepSeekPricing.Enabled || m.Snapshot().Config.InputCostPerMillion != 0 {
		t.Fatal("Ark inherited DeepSeek pricing")
	}
	for _, raw := range [][]byte{mustJSON(t, pub), mustRead(t, filepath.Join(dir, "settings.enc"))} {
		if strings.Contains(string(raw), req.APIKey) {
			t.Fatal("key leaked")
		}
	}
	restored, err := New(m.original, dir, key)
	if err != nil {
		t.Fatal(err)
	}
	restoredList, err := restored.Snapshot().Models.List(context.Background())
	if err != nil || len(restoredList.List) != 3 || restored.Public().Provider != "volcengine" || restored.Snapshot().Config.APIKey != req.APIKey {
		t.Fatal("restart lost model settings", err)
	}
	// A budget-only update keeps the existing key, but is still revalidated.
	r := update(m)
	r.MaxOutputTokens = 4096
	if _, err := m.Save(context.Background(), r, "43"); err != nil {
		t.Fatal("retaining key failed", err)
	}
}

func TestVolcengineRejectsFailedProbeWithoutPublishing(t *testing.T) {
	for _, failure := range []string{"unauthorized", "missing-model", "empty-response", "cancelled"} {
		t.Run(failure, func(t *testing.T) {
			m, dir, _ := fixture(t)
			before := m.Snapshot()
			arkTransport(t, m, func(r *http.Request) (*http.Response, error) {
				var body struct {
					Model string `json:"model"`
				}
				_ = json.NewDecoder(r.Body).Decode(&body)
				status, response := 200, `{"choices":[{"message":{"content":"OK"}}]}`
				if failure == "unauthorized" {
					status, response = 401, "ark-fixture-key private-error"
				}
				if failure == "missing-model" && body.Model == "doubao-fixture" {
					status = 404
				}
				if failure == "empty-response" {
					response = `{"choices":[]}`
				}
				if failure == "cancelled" {
					return nil, context.Canceled
				}
				return &http.Response{StatusCode: status, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(response))}, nil
			})
			for _, save := range []bool{false, true} {
				var err error
				if save {
					_, err = m.Save(context.Background(), arkUpdate(m), "42")
				} else {
					_, err = m.Test(context.Background(), arkUpdate(m))
				}
				if err == nil || strings.Contains(err.Error(), "ark-fixture-key") || strings.Contains(err.Error(), "private-error") {
					t.Fatal("failure hidden or details leaked", err)
				}
				if m.Snapshot() != before || m.Public().Revision != 0 {
					t.Fatal("failed probe changed current runtime")
				}
			}
			if _, err := os.Stat(filepath.Join(dir, "settings.enc")); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("failed probe persisted settings")
			}
		})
	}
}

func TestVolcengineValidationBeforeProbe(t *testing.T) {
	m, _, _ := fixture(t)
	for _, mutate := range []func(*Update){
		func(r *Update) { r.APIKey = "" },
		func(r *Update) { r.BaseURL = "https://api.deepseek.com" },
		func(r *Update) { r.BaseURL = "https://ark.cn-beijing.volces.com/api/plan/v3" },
		func(r *Update) { r.DefaultModel = "" },
		func(r *Update) { r.DefaultModel = "ep-bad/model" },
		func(r *Update) { r.EnabledModels = []string{"doubao-fixture"} },
		func(r *Update) { r.EnabledModels = append(r.EnabledModels, "doubao-fixture") },
	} {
		r := arkUpdate(m)
		mutate(&r)
		if _, err := m.prepare(r); err == nil {
			t.Fatal("invalid Ark settings accepted")
		}
	}
	r := arkUpdate(m)
	r.BaseURL = ""
	r.EnabledModels = nil
	prepared, err := m.prepare(r)
	if err != nil || prepared.BaseURL != provider.VolcengineBaseURL {
		t.Fatal("default address not applied", err)
	}
	snap, err := m.buildSnapshot(prepared)
	if err != nil {
		t.Fatal(err)
	}
	list, err := snap.Models.List(context.Background())
	if err != nil || len(list.List) != 2 {
		t.Fatal("empty allowlist must only expose configured defaults", err)
	}
}
