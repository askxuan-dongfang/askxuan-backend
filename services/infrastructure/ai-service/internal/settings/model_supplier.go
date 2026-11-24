package settings

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/askxuan/ai-service/internal/provider"
	"github.com/google/uuid"
)

type SupplierImportInput struct {
	Provider        string `json:"provider"`
	APIKey          string `json:"apiKey,omitempty"`
	ReuseConnection string `json:"reuseConnection,omitempty"`
	Revision        int64  `json:"revision"`
}

type SupplierImportResult struct {
	Added    int `json:"added"`
	Existing int `json:"existing"`
}

func (r record) languageAPIKey(model modelRecord) string {
	if key := r.ModelSupplierSecrets[model.Provider]; model.UsesSupplierKey && key != "" {
		return key
	}
	return model.APIKey
}

func (m *Manager) DiscoverLanguageModels(ctx context.Context, kind, key string) ([]provider.ModelOption, error) {
	var name string
	var discover func(context.Context, string) ([]provider.ModelOption, error)
	switch kind {
	case provider.OpenCodeGo:
		name, discover = "OpenCode Go", m.openCodeGoModels
	case "deepseek":
		name, discover = "DeepSeek", m.deepSeekModels
	default:
		return nil, errors.New("该供应商不支持模型发现")
	}
	key = strings.TrimSpace(key)
	if !validLanguageAPIKey(key) {
		return nil, fmt.Errorf("请填写有效的 %s API Key", name)
	}
	models, err := discover(ctx, key)
	if err != nil {
		var upstream *provider.HTTPError
		if errors.As(err, &upstream) {
			return nil, fmt.Errorf("%s 模型列表获取失败（HTTP %d）", name, upstream.StatusCode)
		}
		return nil, fmt.Errorf("%s 模型列表获取失败", name)
	}
	return models, nil
}

func (m *Manager) ImportSupplierModels(ctx context.Context, in SupplierImportInput, actor string) (SupplierImportResult, error) {
	var result SupplierImportResult
	var base string
	switch in.Provider {
	case provider.OpenCodeGo:
		base = provider.OpenCodeGoBaseURL
	case "deepseek":
		base = provider.DeepSeekBaseURL
	default:
		return result, errors.New("该供应商不支持批量导入")
	}
	m.mu.RLock()
	current := m.record
	m.mu.RUnlock()
	if current.Revision != in.Revision {
		return result, ErrConflict
	}
	if m.store == nil {
		return result, errors.New("服务器尚未启用持久化配置")
	}
	matchesSupplier := func(kind, url string) bool {
		return kind == in.Provider && (url == base || (kind == "deepseek" && url == base+"/v1"))
	}
	key := strings.TrimSpace(in.APIKey)
	saved := current.ModelSupplierSecrets[in.Provider]
	switch {
	case saved != "":
		if in.ReuseConnection != "" || (key != "" && key != saved) {
			return result, errors.New("此供应商已添加，请使用已保存的连接同步模型")
		}
		key = saved
	case in.ReuseConnection != "":
		if key != "" {
			return result, errors.New("请选择已有连接或填写 API Key")
		}
		_, existing, err := m.LanguageConnection(ctx, in.ReuseConnection)
		if err != nil {
			return result, err
		}
		if !matchesSupplier(existing.Provider, existing.BaseURL) {
			return result, errors.New("请选择同一供应商的官方连接")
		}
		key = existing.APIKey
	}
	options, err := m.DiscoverLanguageModels(ctx, in.Provider, key)
	if err != nil {
		return result, err
	}
	if len(options) == 0 {
		return result, errors.New("此连接暂无可导入的语言模型")
	}

	existingByModel := make(map[string]int)
	for i, model := range current.Models {
		if matchesSupplier(model.Provider, model.BaseURL) && current.languageAPIKey(model) == key {
			existingByModel[model.Model] = i
		}
	}
	legacyMatches := matchesSupplier(current.Provider, current.BaseURL) && current.APIKey == key
	next := current
	next.Models = slices.Clone(current.Models)
	changed := saved == ""
	for _, option := range options {
		if i, found := existingByModel[option.ID]; found {
			result.Existing++
			model := &next.Models[i]
			if !model.UsesSupplierKey {
				model.UsesSupplierKey, model.APIKey = true, ""
				changed = true
			}
			continue
		}
		if legacyMatches && (len(current.EnabledModels) == 0 || slices.Contains(current.EnabledModels, option.ID)) {
			result.Existing++
			continue
		}
		next.Models = append(next.Models, modelRecord{
			ID: "model-" + uuid.NewString(), UsesSupplierKey: true,
			LanguageModelValues: LanguageModelValues{
				Provider: in.Provider, BaseURL: base, Model: option.ID, Name: option.Name,
				SupportsVision: option.SupportsVision, ContextWindow: option.ContextWindow, MaxOutputTokens: option.MaxOutputTokens,
			},
		})
		result.Added++
		changed = true
	}
	if len(next.Models) > maxLanguageModels {
		return SupplierImportResult{}, errors.New("批量导入将超过语言模型登记上限")
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if m.record.Revision != in.Revision {
		return SupplierImportResult{}, ErrConflict
	}
	if changed {
		next.ModelSupplierSecrets = make(map[string]string, len(current.ModelSupplierSecrets)+1)
		maps.Copy(next.ModelSupplierSecrets, current.ModelSupplierSecrets)
		next.ModelSupplierSecrets[in.Provider] = key
		if err := m.saveLibraryLocked(next, in.Revision, actor, "modelLibrary.importSupplier"); err != nil {
			return SupplierImportResult{}, err
		}
	}
	return result, nil
}
