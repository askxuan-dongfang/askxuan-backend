package settings

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/askxuan/ai-service/internal/provider"
)

func TestOpenCodeGoSupplierImportReusesModelIdentityAndSyncs(t *testing.T) {
	ctx := context.Background()
	m, dir, storeKey := libraryFixture(t)
	options := []provider.ModelOption{{ID: "kimi-k3", Name: "Kimi K3", SupportsVision: true}}
	m.openCodeGoModels = func(_ context.Context, key string) ([]provider.ModelOption, error) {
		if key != "fixture-library-key" {
			return nil, &provider.HTTPError{StatusCode: 401}
		}
		return options, nil
	}
	in := languageInput(0)
	in.Provider, in.BaseURL, in.Model = provider.OpenCodeGo, provider.OpenCodeGoBaseURL, "kimi-k3"
	model, err := m.AddLanguageModel(ctx, in, "42")
	if err != nil {
		t.Fatal(err)
	}
	defaults := ModelUses{CatalogTranslation: model.ID}
	if err := m.SaveModelUses(ctx, defaults, nil, 1, "42"); err != nil {
		t.Fatal(err)
	}
	options = append(options, provider.ModelOption{ID: "glm-new", Name: "GLM New"})
	loaded, err := m.ImportSupplierModels(ctx, SupplierImportInput{
		Provider: provider.OpenCodeGo, ReuseConnection: model.ID, Revision: 2,
	}, "42")
	if err != nil || loaded != (SupplierImportResult{Added: 1, Existing: 1}) {
		t.Fatalf("bulk import: %+v, %v", loaded, err)
	}
	uses, _, suppliers, revision := m.ModelLibraryUses()
	if uses != defaults || revision != 3 || len(suppliers) != 1 || suppliers[0] != provider.OpenCodeGo {
		t.Fatalf("default binding or supplier registration lost: %+v, %v, %d", uses, suppliers, revision)
	}
	for _, row := range m.record.Models {
		if !row.UsesSupplierKey || row.APIKey != "" {
			t.Fatal("supplier model did not reuse the shared credential")
		}
	}
	if route := m.Snapshot().CatalogTranslation; route.Option.ID != model.ID || route.Model != "kimi-k3" {
		t.Fatal("previous catalog selection no longer resolves")
	}

	sync := SupplierImportInput{Provider: provider.OpenCodeGo, Revision: 3}
	result, err := m.ImportSupplierModels(ctx, sync, "42")
	if err != nil || result != (SupplierImportResult{Existing: 2}) || m.Public().Revision != 3 {
		t.Fatalf("repeat import changed the settings: %+v %v", result, err)
	}
	options = append(options, provider.ModelOption{ID: "qwen-new", Name: "Qwen New", SupportsVision: true})
	result, err = m.ImportSupplierModels(ctx, sync, "42")
	if err != nil || result != (SupplierImportResult{Added: 1, Existing: 2}) {
		t.Fatalf("sync did not add exactly the new model: %+v %v", result, err)
	}
	if _, err := m.ImportSupplierModels(ctx, sync, "42"); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale revision was accepted: %v", err)
	}
	if _, err := m.Save(ctx, update(m), "42"); err != nil {
		t.Fatal(err)
	}
	restarted, err := New(m.original, dir, storeKey)
	if err != nil {
		t.Fatal(err)
	}
	_, oldRoute, err := restarted.LanguageConnection(ctx, model.ID)
	if err != nil || oldRoute.APIKey != in.APIKey || oldRoute.Model != in.Model ||
		len(restarted.record.ModelSupplierSecrets) != 1 || len(restarted.record.Models) != 3 {
		t.Fatal("registered connection or model IDs did not survive a settings save and restart")
	}

	sealed := mustRead(t, filepath.Join(dir, "settings.enc"))
	nonce := m.store.cipher.NonceSize()
	clear, err := m.store.cipher.Open(nil, sealed[:nonce], sealed[nonce:], []byte("askxuan-ai-settings-v1"))
	if err != nil {
		t.Fatal(err)
	}
	var previous struct {
		Models []struct {
			ID  string `json:"id"`
			Key string `json:"secret"`
		} `json:"catalogModels"`
	}
	if err := json.Unmarshal(clear, &previous); err != nil || len(previous.Models) != 3 {
		t.Fatal("previous binary cannot see the imported OpenCode Go models")
	}
	for _, row := range previous.Models {
		if row.Key != in.APIKey {
			t.Fatal("rollback-compatible model credential missing")
		}
	}
}

func TestDeepSeekSupplierImportKeepsMatchingLegacyModels(t *testing.T) {
	ctx := context.Background()
	m, _, _ := libraryFixture(t)
	m.record.EnabledModels = []string{"deepseek-flash", "deepseek-v4-pro"}
	active, err := m.build(m.record)
	if err != nil {
		t.Fatal(err)
	}
	m.active = active
	m.deepSeekModels = func(_ context.Context, key string) ([]provider.ModelOption, error) {
		if key != "fixture-old-key" {
			return nil, &provider.HTTPError{StatusCode: 401}
		}
		return []provider.ModelOption{
			{ID: "deepseek-flash", Name: "DeepSeek Flash", SupportsVision: true},
			{ID: "deepseek-v4-pro", Name: "DeepSeek V4 Pro"},
			{ID: "deepseek-fixture-vision", Name: "DeepSeek Fixture Vision", SupportsVision: true, ContextWindow: 131072},
		}, nil
	}

	in := SupplierImportInput{Provider: "deepseek", APIKey: "invalid-fixture-key"}
	if _, err := m.ImportSupplierModels(ctx, in, "42"); err == nil || m.Public().Revision != 0 || len(m.record.ModelSupplierSecrets) != 0 {
		t.Fatal("supplier import changed the library after a failed discovery")
	}
	in.APIKey, in.ReuseConnection = "", "deepseek-flash"
	result, err := m.ImportSupplierModels(ctx, in, "42")
	if err != nil || result != (SupplierImportResult{Added: 1, Existing: 2}) || len(m.record.Models) != 1 {
		t.Fatalf("existing DeepSeek models were duplicated: %+v, %v", result, err)
	}
	added := m.record.Models[0]
	if added.Model != "deepseek-fixture-vision" || !added.SupportsVision || added.ContextWindow != 131072 {
		t.Fatal("imported model lost its advertised capabilities")
	}
	runtime := m.Snapshot()
	if runtime.Provider.Model() != "deepseek-flash" {
		t.Fatal("import changed the running chat default")
	}
	if selected, err := runtime.Models.Select(ctx, added.ID, true); err != nil || selected != added.ID {
		t.Fatalf("DeepSeek import missing from the vision selector: %q %v", selected, err)
	}
}

func TestSupplierImportRejectsBatchAboveExistingLimit(t *testing.T) {
	m, _, _ := libraryFixture(t)
	m.deepSeekModels = func(context.Context, string) ([]provider.ModelOption, error) {
		return []provider.ModelOption{{ID: "new-text", Name: "New Text"}}, nil
	}
	m.record.Models = make([]modelRecord, maxLanguageModels)
	_, err := m.ImportSupplierModels(context.Background(), SupplierImportInput{Provider: "deepseek", APIKey: "fixture-supplier-key"}, "42")
	if err == nil || len(m.record.Models) != maxLanguageModels || len(m.record.ModelSupplierSecrets) != 0 || m.Public().Revision != 0 {
		t.Fatal("over-limit batch did not fail atomically")
	}
}
