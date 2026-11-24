package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/askxuan/ai-service/internal/provider"
	"github.com/google/uuid"
)

type ModelUses struct {
	Chat               string `json:"chat"`
	Vision             string `json:"vision"`
	Embedding          string `json:"embedding"`
	Rerank             string `json:"rerank"`
	Document           string `json:"document"`
	CatalogTranslation string `json:"catalogTranslation"`
}

type LanguageModelValues struct {
	Name            string `json:"name"`
	Model           string `json:"model"`
	Provider        string `json:"provider"`
	BaseURL         string `json:"baseUrl"`
	SupportsVision  bool   `json:"supportsVision"`
	ContextWindow   int    `json:"contextWindow"`
	MaxOutputTokens int    `json:"maxOutputTokens"`
}

type LanguageModel struct {
	LanguageModelValues
	ID              string `json:"id"`
	Source          string `json:"source"`
	DocumentModelID string `json:"documentModelId,omitempty"`
}

type LanguageModelInput struct {
	LanguageModelValues
	Revision        int64  `json:"revision"`
	APIKey          string `json:"apiKey"`
	ReuseConnection string `json:"reuseConnection,omitempty"`
}

type modelRecord struct {
	LanguageModelValues
	ID              string `json:"id"`
	APIKey          string `json:"secret,omitempty"`
	UsesSupplierKey bool   `json:"usesSupplierKey,omitempty"`
}

const maxLanguageModels = 100

func copyLinks(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func buildLanguageModel(r modelRecord) (provider.Provider, provider.ModelOption, error) {
	kind := r.Provider
	if kind == "deepseek" {
		kind = "openai_compatible"
	}
	p, err := provider.New(provider.Config{Provider: kind, BaseURL: r.BaseURL, APIKey: r.APIKey, Model: r.Model})
	if err != nil {
		return nil, provider.ModelOption{}, err
	}
	if compatible, ok := p.(*provider.OpenAICompatible); ok {
		compatible.SetHTTPClient(secureClient())
		if r.Provider == "openai_compatible" {
			compatible.UseStandardParameters()
		}
	}
	option := provider.ModelOption{ID: r.ID, Name: r.Name, Description: r.Model, SupportsVision: r.SupportsVision, ContextWindow: r.ContextWindow, MaxOutputTokens: r.MaxOutputTokens}
	if r.Provider == provider.OpenCodeGo {
		return p, option, nil
	}
	// Never inflate a known Plan model beyond its documented capacity.
	if r.Provider == "volcengine" {
		list, err := provider.NewCatalog(p).List(context.Background())
		if err != nil {
			return nil, option, err
		}
		for _, known := range list.List {
			if known.ContextWindow > 0 && (option.ContextWindow == 0 || known.ContextWindow < option.ContextWindow) {
				option.ContextWindow = known.ContextWindow
			}
			if known.MaxOutputTokens > 0 && (option.MaxOutputTokens == 0 || known.MaxOutputTokens < option.MaxOutputTokens) {
				option.MaxOutputTokens = known.MaxOutputTokens
			}
		}
	}
	if option.ContextWindow == 0 {
		option.ContextWindow = 32768
	}
	if option.MaxOutputTokens == 0 {
		option.MaxOutputTokens = 8192
	}
	return p, option, nil
}

func (m *Manager) withModelLibrary(snap *Snapshot, r record) (*Snapshot, error) {
	snap.CustomerModelIDs = slices.Clone(r.CustomerModelIDs)
	snap.CatalogTranslation.Model = r.ModelUses.CatalogTranslation
	routes := make([]provider.RegisteredRoute, 0, len(r.Models))
	for _, registered := range r.Models {
		registered.APIKey = r.languageAPIKey(registered)
		p, option, err := m.modelBuilder(registered)
		if err != nil {
			return nil, err
		}
		route := provider.RegisteredRoute{
			Option: option, Provider: p, Model: registered.Model,
			ExplicitCustomerChoice: registered.Provider == provider.OpenCodeGo,
		}
		if route.ExplicitCustomerChoice && registered.ID == r.ModelUses.CatalogTranslation {
			snap.CatalogTranslation = route
		}
		routes = append(routes, route)
	}
	if len(routes) == 0 && r.ModelUses.Chat == "" && r.ModelUses.Vision == "" {
		return snap, nil
	}
	snap.Provider = provider.NewRegistry(snap.Provider, r.EnabledModels, routes, r.ModelUses.Chat, r.ModelUses.Vision)
	snap.Models = provider.NewCatalog(snap.Provider)
	snap.Config.Model = snap.Provider.Model()
	return snap, nil
}

func (m *Manager) ModelUses() (ModelUses, int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.record.ModelUses, m.record.Revision
}

func (m *Manager) ModelLibraryUses() (ModelUses, []string, []string, int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	suppliers := slices.AppendSeq([]string{}, maps.Keys(m.record.ModelSupplierSecrets))
	slices.Sort(suppliers)
	return m.record.ModelUses, append([]string{}, m.record.CustomerModelIDs...), suppliers, m.record.Revision
}

func (m *Manager) ValidateCustomerModelIDs(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	list, err := m.Snapshot().Models.CustomerList(ctx, ids)
	if err != nil {
		return err
	}
	if len(list.List) != len(ids) {
		return errors.New("信众端开放模型不可用或重复，请从对话模型中选择")
	}
	return nil
}

func (m *Manager) LanguageModels(ctx context.Context) ([]LanguageModel, error) {
	m.mu.RLock()
	r, snapshot := m.record, m.active
	m.mu.RUnlock()
	list, err := provider.NewCatalogWithAllowed(provider.PrimaryProvider(snapshot.Provider), r.EnabledModels).List(ctx)
	out := []LanguageModel{}
	for _, option := range list.List {
		if option.ID == "mock" {
			continue
		}
		out = append(out, LanguageModel{ID: option.ID, Source: "legacy", DocumentModelID: r.KnowledgeLinks[option.ID], LanguageModelValues: LanguageModelValues{Name: option.Name, Model: option.ID, Provider: r.Provider, BaseURL: r.BaseURL, SupportsVision: option.SupportsVision, ContextWindow: option.ContextWindow, MaxOutputTokens: option.MaxOutputTokens}})
	}
	for _, v := range r.Models {
		out = append(out, LanguageModel{ID: v.ID, Source: "registered", DocumentModelID: r.KnowledgeLinks[v.ID], LanguageModelValues: v.LanguageModelValues})
	}
	return out, err
}

// LanguageConnection is server-only. Public handlers must return LanguageModel
// instead; credentials never enter the model directory or browser responses.
func (m *Manager) LanguageConnection(ctx context.Context, id string) (LanguageModel, provider.Config, error) {
	m.mu.RLock()
	r, snapshot := m.record, m.active
	m.mu.RUnlock()
	for _, v := range r.Models {
		if v.ID == id {
			return LanguageModel{ID: id, Source: "registered", DocumentModelID: r.KnowledgeLinks[id], LanguageModelValues: v.LanguageModelValues}, provider.Config{Provider: v.Provider, BaseURL: v.BaseURL, APIKey: r.languageAPIKey(v), Model: v.Model}, nil
		}
	}
	list, err := provider.NewCatalogWithAllowed(provider.PrimaryProvider(snapshot.Provider), r.EnabledModels).List(ctx)
	if err != nil {
		return LanguageModel{}, provider.Config{}, errors.New("原有连接暂不可用，请测试连接")
	}
	for _, option := range list.List {
		if option.ID == id && id != "mock" {
			v := LanguageModel{ID: id, Source: "legacy", DocumentModelID: r.KnowledgeLinks[id], LanguageModelValues: LanguageModelValues{Name: option.Name, Model: id, Provider: r.Provider, BaseURL: r.BaseURL, SupportsVision: option.SupportsVision, ContextWindow: option.ContextWindow, MaxOutputTokens: option.MaxOutputTokens}}
			return v, provider.Config{Provider: r.Provider, BaseURL: r.BaseURL, APIKey: r.APIKey, Model: id}, nil
		}
	}
	return LanguageModel{}, provider.Config{}, provider.ErrModelUnavailable
}

func (m *Manager) prepareLanguageModel(ctx context.Context, in LanguageModelInput) (modelRecord, error) {
	r := modelRecord{ID: "model-" + uuid.NewString(), LanguageModelValues: in.LanguageModelValues, APIKey: strings.TrimSpace(in.APIKey)}
	r.Name, r.Model = strings.TrimSpace(r.Name), strings.TrimSpace(r.Model)
	if r.Name == "" {
		r.Name = r.Model
	}
	if len(r.Name) > 120 || !validModel.MatchString(r.Model) {
		return r, errors.New("请填写有效的模型名称与服务商模型 ID")
	}
	if r.Provider == provider.OpenCodeGo && strings.TrimSpace(r.BaseURL) == "" {
		r.BaseURL = provider.OpenCodeGoBaseURL
	}
	base, err := normalizeURL(r.BaseURL)
	if err != nil {
		return r, err
	}
	r.BaseURL = base
	u, _ := url.Parse(base)
	if r.Provider != "deepseek" && r.Provider != "volcengine" && r.Provider != "openai_compatible" && r.Provider != provider.OpenCodeGo {
		return r, errors.New("不支持的语言模型供应商")
	}
	if r.Provider == "deepseek" && (u.Hostname() != "api.deepseek.com" || u.Path != "" && u.Path != "/v1") {
		return r, errors.New("DeepSeek 使用官方接口地址")
	}
	if r.Provider == "volcengine" && !provider.IsVolcengineURL(base) {
		return r, errors.New("请填写火山方舟模型服务或 Agent Plan 官方根地址")
	}
	if r.Provider == provider.OpenCodeGo && base != provider.OpenCodeGoBaseURL {
		return r, errors.New("OpenCode Go 使用固定的官方接口地址")
	}
	if r.Provider != provider.OpenCodeGo && (r.ContextWindow != 0 && (r.ContextWindow < 32768 || r.ContextWindow > 1048576) ||
		r.MaxOutputTokens != 0 && (r.MaxOutputTokens < 64 || r.MaxOutputTokens > 32768)) {
		return r, errors.New("请核对模型上下文与输出容量")
	}
	if r.APIKey == "" && in.ReuseConnection != "" {
		_, old, err := m.LanguageConnection(ctx, in.ReuseConnection)
		if err != nil {
			return r, err
		}
		if old.Provider != r.Provider || old.BaseURL != r.BaseURL {
			return r, errors.New("只能复用同一供应商与地址的连接密钥")
		}
		r.APIKey = old.APIKey
	}
	if r.Provider == provider.OpenCodeGo {
		models, err := m.DiscoverLanguageModels(ctx, r.Provider, r.APIKey)
		if err != nil {
			return r, err
		}
		for _, model := range models {
			if model.ID == r.Model {
				r.ContextWindow, r.MaxOutputTokens, r.SupportsVision = model.ContextWindow, model.MaxOutputTokens, model.SupportsVision
				return r, nil
			}
		}
		return r, errors.New("请从当前密钥可用的 OpenCode Go 文本模型中选择")
	}
	if !validLanguageAPIKey(r.APIKey) {
		return r, errors.New("请填写 API Key 或选择已有连接")
	}
	return r, nil
}

func validLanguageAPIKey(key string) bool {
	return key != "" && len(key) <= 4096 && !strings.ContainsAny(key, "\r\n\t ")
}

func (m *Manager) checkLanguageModel(ctx context.Context, r modelRecord) (modelRecord, error) {
	p, option, err := m.modelBuilder(r)
	if err != nil {
		return r, errors.New("模型连接配置无效")
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	if r.Provider == provider.OpenCodeGo {
		budget := 768
		if r.MaxOutputTokens > 0 {
			budget = min(budget, r.MaxOutputTokens)
		}
		reply, err := p.Complete(ctx, provider.Request{Model: r.Model, SessionID: "askxuan-probe-" + r.ID,
			JSONMode: true, MaxTokens: budget, SystemPrompt: `Return only this JSON object: {"ok":true}.`,
			Messages: []provider.Message{{Role: "user", Content: "Reply with the JSON confirmation."}},
		})
		if err != nil {
			var upstream *provider.HTTPError
			if errors.As(err, &upstream) {
				return r, fmt.Errorf("OpenCode Go 模型测试失败（HTTP %d）", upstream.StatusCode)
			}
			return r, errors.New("OpenCode Go 模型连接测试失败")
		}
		var probe struct {
			OK bool `json:"ok"`
		}
		if reply == nil || reply.FinishReason == "length" || json.Unmarshal([]byte(reply.Content), &probe) != nil || !probe.OK {
			return r, errors.New("OpenCode Go 模型未返回可用的文本 JSON")
		}
		return r, nil
	}
	r.ContextWindow, r.MaxOutputTokens = option.ContextWindow, min(32768, option.MaxOutputTokens)
	message := provider.Message{Role: "user", Content: "Reply with OK only."}
	if r.SupportsVision {
		message.ImageDataURLs = []string{probeImage}
	}
	_, err = p.Complete(ctx, provider.Request{Model: r.Model, Messages: []provider.Message{message}, MaxTokens: 512})
	if err != nil {
		return r, errors.New("模型连接测试失败：" + strings.ReplaceAll(volcengineProbeFailure(err), "火山方舟", "模型服务"))
	}
	return r, nil
}

func (m *Manager) TestLanguageModel(ctx context.Context, in LanguageModelInput) (LanguageModel, error) {
	r, err := m.prepareLanguageModel(ctx, in)
	if err == nil {
		r, err = m.checkLanguageModel(ctx, r)
	}
	return LanguageModel{LanguageModelValues: r.LanguageModelValues}, err
}

func (m *Manager) AddLanguageModel(ctx context.Context, in LanguageModelInput, actor string) (LanguageModel, error) {
	if _, revision := m.ModelUses(); revision != in.Revision {
		return LanguageModel{}, ErrConflict
	}
	r, err := m.prepareLanguageModel(ctx, in)
	if err == nil {
		r, err = m.checkLanguageModel(ctx, r)
	}
	if err != nil {
		return LanguageModel{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.record.Models) >= maxLanguageModels {
		return LanguageModel{}, errors.New("已登记语言模型达到上限")
	}
	for _, old := range m.record.Models {
		if old.Provider == r.Provider && old.BaseURL == r.BaseURL && old.Model == r.Model && m.record.languageAPIKey(old) == r.APIKey {
			return LanguageModel{}, errors.New("此连接的模型已登记，请从模型库选择")
		}
	}
	next := m.record
	next.Models = append(append([]modelRecord{}, next.Models...), r)
	if err = m.saveLibraryLocked(next, in.Revision, actor, "modelLibrary.add"); err != nil {
		return LanguageModel{}, err
	}
	return LanguageModel{ID: r.ID, Source: "registered", LanguageModelValues: r.LanguageModelValues}, nil
}

func (m *Manager) saveLibraryLocked(r record, expected int64, actor, field string) error {
	if m.store == nil {
		return errors.New("服务器尚未启用持久化配置")
	}
	if m.record.Revision != expected {
		return ErrConflict
	}
	snapshot, err := m.build(r)
	if err != nil {
		return errors.New("模型目录无法加载，原配置保持不变")
	}
	r.Revision++
	r.History = append([]Audit{{Revision: r.Revision, Actor: actor, At: time.Now().UTC().Format(time.RFC3339), Fields: []string{field}}}, m.record.History...)
	if len(r.History) > 50 {
		r.History = r.History[:50]
	}
	if err = m.store.write(r, expected); err != nil {
		return err
	}
	m.record, m.active = r, snapshot
	return nil
}

func (m *Manager) SaveModelUses(ctx context.Context, uses ModelUses, customerModelIDs []string, revision int64, actor string) error {
	m.mu.RLock()
	snapshot := m.active
	m.mu.RUnlock()
	for _, choice := range []struct {
		id     string
		vision bool
	}{{uses.Chat, false}, {uses.Vision, true}, {uses.CatalogTranslation, false}} {
		if choice.id != "" {
			if _, err := snapshot.Models.Select(ctx, choice.id, choice.vision); err != nil {
				return err
			}
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.record
	r.ModelUses = uses
	if customerModelIDs != nil {
		r.CustomerModelIDs = slices.Clone(customerModelIDs)
	}
	return m.saveLibraryLocked(r, revision, actor, "modelLibrary.uses")
}

func (m *Manager) LinkKnowledgeModel(id, engineID, actor string, revision int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.record
	r.KnowledgeLinks = copyLinks(r.KnowledgeLinks)
	r.KnowledgeLinks[id] = engineID
	return m.saveLibraryLocked(r, revision, actor, "modelLibrary.document")
}

// The caller must also reject references from knowledge bases and frozen agent
// versions. In-flight requests retain their already captured provider snapshot.
func (m *Manager) DeleteLanguageModel(id, actor string, revision int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := m.record
	if id == r.ModelUses.Chat || id == r.ModelUses.Vision || id == r.ModelUses.CatalogTranslation {
		return errors.New("默认用途正在使用此模型，请先选择其他模型")
	}
	if slices.Contains(r.CustomerModelIDs, id) {
		return errors.New("此模型已向信众端开放，请先取消开放")
	}
	r.Models = []modelRecord{}
	found := false
	for _, v := range m.record.Models {
		if v.ID == id {
			found = true
		} else {
			r.Models = append(r.Models, v)
		}
	}
	if !found {
		return errors.New("原有连接模型请在连接设置中管理")
	}
	r.KnowledgeLinks = copyLinks(r.KnowledgeLinks)
	delete(r.KnowledgeLinks, id)
	return m.saveLibraryLocked(r, revision, actor, "modelLibrary.delete")
}

// Engine adapters and saved defaults must remain available for existing bindings.
func (m *Manager) KnowledgeModelInUse(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u := m.record.ModelUses
	if id == u.Document || id == u.Embedding || id == u.Rerank {
		return true
	}
	for _, linked := range m.record.KnowledgeLinks {
		if linked == id {
			return true
		}
	}
	return false
}
