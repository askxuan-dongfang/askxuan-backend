package settings

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/askxuan/ai-service/internal/provider"
)

func libraryFixture(t *testing.T) (*Manager, string, string) {
	m, dir, key := fixture(t)
	build := m.build
	m.build = func(r record) (*Snapshot, error) {
		s, e := build(r)
		if e != nil {
			return nil, e
		}
		return m.withModelLibrary(s, r)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-library-key" {
			http.Error(w, "private diagnostic fixture-library-key", 401)
			return
		}
		var v struct {
			Model          string            `json:"model"`
			ResponseFormat map[string]string `json:"response_format"`
		}
		_ = json.NewDecoder(r.Body).Decode(&v)
		text := "OK"
		if v.ResponseFormat["type"] == "json_object" {
			text = `{"ok":true}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"model": v.Model, "choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": text}}}})
	}))
	t.Cleanup(server.Close)
	m.modelBuilder = func(r modelRecord) (provider.Provider, provider.ModelOption, error) {
		return provider.NewOpenAICompatible(server.URL, r.APIKey, r.Model, ""), provider.ModelOption{ID: r.ID, Name: r.Name, SupportsVision: r.SupportsVision, ContextWindow: 65536, MaxOutputTokens: 8192}, nil
	}
	return m, dir, key
}
func languageInput(revision int64) LanguageModelInput {
	return LanguageModelInput{Revision: revision, APIKey: "fixture-library-key", LanguageModelValues: LanguageModelValues{Name: "Library model", Model: "same-name", Provider: "openai_compatible", BaseURL: "https://models.example.com/v1"}}
}

func TestOpenCodeGoModelsAreOnlyAvailableToCatalogTranslation(t *testing.T) {
	ctx := context.Background()
	m, dir, key := libraryFixture(t)
	m.original.ReportModel = "configured-report"
	discoveries := 0
	m.openCodeGoModels = func(_ context.Context, credential string) ([]provider.ModelOption, error) {
		discoveries++
		if credential != "fixture-library-key" {
			return nil, &provider.HTTPError{StatusCode: 403}
		}
		return []provider.ModelOption{{ID: "kimi-k3", Name: "Kimi K3", ContextWindow: 2097152, MaxOutputTokens: 1048576, SupportsVision: true}}, nil
	}
	in := languageInput(0)
	in.Provider, in.Model, in.BaseURL = provider.OpenCodeGo, "kimi-k3", "https://elsewhere.example/v1"
	if _, err := m.AddLanguageModel(ctx, in, "42"); err == nil || discoveries != 0 {
		t.Fatal("off-host credential was accepted")
	}
	in.BaseURL, in.Model = provider.OpenCodeGoBaseURL, "unlisted"
	if _, err := m.AddLanguageModel(ctx, in, "42"); err == nil || m.Public().Revision != 0 {
		t.Fatal("unlisted OpenCode Go model was registered")
	}
	in.Model, in.ContextWindow, in.MaxOutputTokens, in.SupportsVision = "kimi-k3", 100, 100, false
	model, err := m.AddLanguageModel(ctx, in, "42")
	if err != nil {
		t.Fatal(err)
	}
	if model.ContextWindow != 2097152 || model.MaxOutputTokens != 1048576 || !model.SupportsVision {
		t.Fatalf("provider limits or metadata were replaced by browser values: %+v", model)
	}
	if m.Snapshot().CatalogTranslation.Provider != nil {
		t.Fatal("unbound OpenCode Go model was activated")
	}
	if _, err := m.Snapshot().Models.Select(ctx, model.ID, false); !errors.Is(err, provider.ErrModelUnavailable) {
		t.Fatalf("customer can select catalog-only Go model: %v", err)
	}
	if err := m.SaveModelUses(ctx, ModelUses{Chat: model.ID}, 1, "42"); err == nil {
		t.Fatal("OpenCode Go was accepted as chat default")
	}
	if err := m.SaveModelUses(ctx, ModelUses{Vision: model.ID}, 1, "42"); err == nil {
		t.Fatal("OpenCode Go was accepted as vision default")
	}
	if err := m.SaveModelUses(ctx, ModelUses{CatalogTranslation: model.ID}, 1, "42"); err != nil {
		t.Fatal(err)
	}
	snapshot := m.Snapshot()
	if snapshot.Provider.Model() != "deepseek-flash" || snapshot.Config.ReportModel != "configured-report" ||
		snapshot.CatalogTranslation.Provider == nil || snapshot.CatalogTranslation.Model != "kimi-k3" {
		t.Fatalf("unexpected provider isolation: %+v", snapshot.CatalogTranslation)
	}
	if _, err := snapshot.Models.Select(ctx, model.ID, false); !errors.Is(err, provider.ErrModelUnavailable) {
		t.Fatalf("bound catalog model reached customer picker: %v", err)
	}
	restarted, err := New(m.original, dir, key)
	if err != nil {
		t.Fatal(err)
	}
	if route := restarted.Snapshot().CatalogTranslation; route.Provider == nil || route.Provider.Name() != provider.OpenCodeGo ||
		route.Option.MaxOutputTokens != 1048576 {
		t.Fatal("catalog binding or authoritative limits lost after restart")
	}
	if err := m.DeleteLanguageModel(model.ID, "42", 2); err == nil {
		t.Fatal("deleted active catalog model")
	}
	if err := m.SaveModelUses(ctx, ModelUses{}, 2, "42"); err != nil {
		t.Fatal(err)
	}
	if route := m.Snapshot().CatalogTranslation; route.Provider != nil || route.Model != "" {
		t.Fatal("clearing the binding did not restore current text routing")
	}
}

func TestStoredOpenCodeGoModelsDoNotReachLegacyRuntime(t *testing.T) {
	m, dir, _ := fixture(t)
	history := Audit{Revision: 1, Actor: "42", Fields: []string{"modelLibrary.uses"}}
	r := m.record
	r.Revision = 1
	r.History = []Audit{history}
	r.Models = []modelRecord{
		{ID: "existing", LanguageModelValues: LanguageModelValues{Provider: "openai_compatible"}},
		{ID: "go-catalog", LanguageModelValues: LanguageModelValues{Provider: provider.OpenCodeGo}},
	}
	r.ModelUses = ModelUses{Chat: "existing", CatalogTranslation: "go-catalog"}
	if err := m.store.write(r, 0); err != nil {
		t.Fatal(err)
	}
	sealed := mustRead(t, filepath.Join(dir, "settings.enc"))
	nonce := m.store.cipher.NonceSize()
	plain, err := m.store.cipher.Open(nil, sealed[:nonce], sealed[nonce:], []byte("askxuan-ai-settings-v1"))
	if err != nil {
		t.Fatal(err)
	}
	var legacy struct {
		Revision  int64 `json:"revision"`
		ModelUses struct {
			Chat string `json:"chat"`
		} `json:"modelUses"`
		Models []modelRecord `json:"models"`
	}
	if err := json.Unmarshal(plain, &legacy); err != nil || legacy.Revision != 1 || legacy.ModelUses.Chat != "existing" ||
		len(legacy.Models) != 1 || legacy.Models[0].ID != "existing" || legacy.Models[0].Provider != "openai_compatible" {
		t.Fatalf("legacy settings include unsupported models or lost the existing chat binding: %+v, %v", legacy, err)
	}
	reopened, err := m.store.read()
	if err != nil || reopened.ModelUses.CatalogTranslation != "go-catalog" || len(reopened.Models) != 2 || reopened.Models[1].ID != "go-catalog" {
		t.Fatal("new runtime lost the catalog-only model after reopening")
	}
}

func TestLibraryPersistenceRedactionCASAndLegacySave(t *testing.T) {
	ctx := context.Background()
	m, dir, key := libraryFixture(t)
	before := m.Snapshot()
	model, err := m.AddLanguageModel(ctx, languageInput(0), "42")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.AddLanguageModel(ctx, languageInput(0), "43"); !errors.Is(err, ErrConflict) {
		t.Fatal("stale edit accepted", err)
	}
	uses := ModelUses{Chat: model.ID, Embedding: "embedding", Rerank: "rank"}
	if err = m.SaveModelUses(ctx, uses, 1, "42"); err != nil {
		t.Fatal(err)
	}
	if before.Provider.Model() != "deepseek-flash" || m.Snapshot().Provider.Model() != model.ID {
		t.Fatal("snapshot isolation failed")
	}
	if err = m.DeleteLanguageModel(model.ID, "42", 2); err == nil {
		t.Fatal("deleted default model")
	}
	if err = m.LinkKnowledgeModel(model.ID, "document-engine", "42", 2); err != nil {
		t.Fatal(err)
	}
	if !m.KnowledgeModelInUse("document-engine") || !m.KnowledgeModelInUse("rank") {
		t.Fatal("lost engine binding")
	}
	values, err := m.LanguageModels(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range [][]byte{mustJSON(t, model), mustJSON(t, values), mustJSON(t, m.Public()), mustRead(t, filepath.Join(dir, "settings.enc"))} {
		if strings.Contains(string(raw), "fixture-library-key") {
			t.Fatal("credential escaped")
		}
	}
	// The older provider/search editor cannot erase the independent model list.
	req := update(m)
	req.MaxOutputTokens = 4096
	if _, err = m.Save(ctx, req, "42"); err != nil {
		t.Fatal(err)
	}
	if len(m.record.Models) != 1 || m.record.KnowledgeLinks[model.ID] != "document-engine" || m.record.ModelUses != uses {
		t.Fatal("legacy save erased model library")
	}
	restarted, err := New(m.original, dir, key)
	if err != nil {
		t.Fatal(err)
	}
	if restarted.Snapshot().Provider.Model() != model.ID || len(restarted.record.Models) != 1 || restarted.record.Models[0].APIKey != "fixture-library-key" {
		t.Fatal("library did not survive restart")
	}
}

func TestLibraryProbeReuseAndFailedWrites(t *testing.T) {
	ctx := context.Background()
	m, _, _ := libraryFixture(t)
	if _, err := m.TestLanguageModel(ctx, languageInput(0)); err != nil {
		t.Fatal(err)
	}
	if m.Public().Revision != 0 || len(m.record.Models) != 0 {
		t.Fatal("probe persisted draft")
	}
	model, err := m.AddLanguageModel(ctx, languageInput(0), "1")
	if err != nil {
		t.Fatal(err)
	}
	input := languageInput(1)
	input.APIKey = ""
	input.ReuseConnection = model.ID
	input.Model = "second-model"
	if _, err = m.AddLanguageModel(ctx, input, "1"); err != nil {
		t.Fatal(err)
	}
	input.Revision = 2
	input.BaseURL = "https://attacker.example.com/v1"
	if _, err = m.TestLanguageModel(ctx, input); err == nil {
		t.Fatal("credential reused for another host")
	}
	input = languageInput(2)
	input.APIKey = "bad-key"
	input.Model = "third-model"
	if _, err = m.AddLanguageModel(ctx, input, "1"); err == nil || strings.Contains(err.Error(), "private diagnostic") {
		t.Fatal("unsafe provider error", err)
	}
	input = languageInput(2)
	input.Model = "third-model"
	m.store.dir = filepath.Join(t.TempDir(), "missing")
	if _, err = m.AddLanguageModel(ctx, input, "1"); err == nil {
		t.Fatal("failed persistence accepted")
	}
	if m.Public().Revision != 2 || len(m.record.Models) != 2 {
		t.Fatal("failed write mutated runtime")
	}
}

func TestLibraryConcurrentAddAndCapabilityValidation(t *testing.T) {
	ctx := context.Background()
	m, _, _ := libraryFixture(t)
	var wg sync.WaitGroup
	result := make(chan error, 2)
	for _, id := range []string{"a", "b"} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			in := languageInput(0)
			in.Model = id
			_, err := m.AddLanguageModel(ctx, in, "1")
			result <- err
		}(id)
	}
	wg.Wait()
	close(result)
	success := 0
	for err := range result {
		if err == nil {
			success++
		} else if !errors.Is(err, ErrConflict) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatal("concurrent writes overwrote", success)
	}
	id := m.record.Models[0].ID
	if err := m.SaveModelUses(ctx, ModelUses{Vision: id}, 1, "1"); !errors.Is(err, provider.ErrModelNeedsVision) {
		t.Fatal("text model accepted for vision", err)
	}
	if err := m.SaveModelUses(ctx, ModelUses{Chat: "unknown"}, 1, "1"); !errors.Is(err, provider.ErrModelUnavailable) {
		t.Fatal("unregistered model accepted", err)
	}
}
