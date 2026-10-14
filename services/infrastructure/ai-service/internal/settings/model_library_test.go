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
			Model string `json:"model"`
		}
		_ = json.NewDecoder(r.Body).Decode(&v)
		_ = json.NewEncoder(w).Encode(map[string]any{"model": v.Model, "choices": []any{map[string]any{"message": map[string]string{"role": "assistant", "content": "OK"}}}})
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
