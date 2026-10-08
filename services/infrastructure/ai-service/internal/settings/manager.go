package settings

import (
	"context"
	"errors"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/askxuan/ai-service/internal/config"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/websearch"
)

var ErrConflict = errors.New("配置已被其他管理员更新，请重新加载后再保存")
var validModel = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:-]{0,99}$`)

type Values struct {
	WebSearchFallbackProvider string             `json:"webSearchFallbackProvider"`
	WebSearchOptions          *websearch.Options `json:"webSearchOptions,omitempty"`
	WebSearchProvider         string             `json:"webSearchProvider"`
	ComplexOutputTokens       int                `json:"complexOutputTokens"`
	ContextWindow             int                `json:"contextWindow"`
	MaxInputChars             int                `json:"maxInputChars"`
	TaskTimeoutSeconds        int                `json:"taskTimeoutSeconds"`

	Provider        string   `json:"provider"`
	BaseURL         string   `json:"baseUrl"`
	DefaultModel    string   `json:"defaultModel"`
	VisionModel     string   `json:"visionModel"`
	EnabledModels   []string `json:"enabledModels"`
	ThinkingEnabled bool     `json:"thinkingEnabled"`
	ReasoningEffort string   `json:"reasoningEffort"`
	MaxOutputTokens int      `json:"maxOutputTokens"`
}
type Update struct {
	WebSearchFallbackAPIKey   string `json:"webSearchFallbackApiKey"`
	ClearWebSearchFallbackKey bool   `json:"clearWebSearchFallbackKey"`
	WebSearchTestTarget       string `json:"webSearchTestTarget,omitempty"`
	WebSearchAPIKey           string `json:"webSearchApiKey"`
	ClearWebSearchKey         bool   `json:"clearWebSearchKey"`
	Values
	Revision int64  `json:"revision"`
	APIKey   string `json:"apiKey"`
}
type Audit struct {
	Revision int64    `json:"revision"`
	Actor    string   `json:"actor"`
	At       string   `json:"at"`
	Fields   []string `json:"fields"`
}
type Public struct {
	HasWebSearchFallbackKey bool `json:"hasWebSearchFallbackKey"`
	HasWebSearchKey         bool `json:"hasWebSearchKey"`
	Values
	Revision  int64   `json:"revision"`
	HasAPIKey bool    `json:"hasApiKey"`
	Writable  bool    `json:"writable"`
	Source    string  `json:"source"`
	History   []Audit `json:"history"`
}
type record struct {
	WebSearchFallbackAPIKey string `json:"webSearchFallbackSecret,omitempty"`
	WebSearchAPIKey         string `json:"webSearchSecret,omitempty"`
	Values
	APIKey   string  `json:"secret"`
	Revision int64   `json:"revision"`
	History  []Audit `json:"history"`
}
type Snapshot struct {
	Config   config.AIConf
	Provider provider.Provider
	Models   *provider.Catalog
}
type Manager struct {
	mu       sync.RWMutex
	original config.AIConf
	active   *Snapshot
	record   record
	store    *store
	build    func(record) (*Snapshot, error)
}

func New(initial config.AIConf, dir, key string) (*Manager, error) {
	st, err := newStore(dir, key)
	if err != nil {
		return nil, err
	}
	kind := initial.Provider
	if u, e := url.Parse(initial.BaseURL); e == nil && u.Hostname() == "api.deepseek.com" {
		kind = "deepseek"
	}
	r := record{WebSearchFallbackAPIKey: initial.WebSearch.FallbackAPIKey, WebSearchAPIKey: initial.WebSearch.APIKey, Values: Values{WebSearchFallbackProvider: initial.WebSearch.FallbackProvider, WebSearchOptions: &initial.WebSearch.Options, WebSearchProvider: initial.WebSearch.Provider, Provider: kind, BaseURL: initial.BaseURL, DefaultModel: initial.Model, VisionModel: initial.VisionModel, ThinkingEnabled: initial.ThinkingEnabled, ReasoningEffort: initial.ReasoningEffort, MaxOutputTokens: initial.MaxOutputTokens, ComplexOutputTokens: initial.ComplexOutputTokens, ContextWindow: initial.ContextWindow, MaxInputChars: initial.MaxInputChars, TaskTimeoutSeconds: initial.TaskTimeoutSeconds}, APIKey: initial.APIKey, History: []Audit{}}
	m := &Manager{original: initial, store: st}
	m.build = m.buildSnapshot
	if st != nil {
		saved, e := st.read()
		if e != nil {
			return nil, e
		}
		if saved != nil {
			r = *saved
		}
	}
	r.Values = normalized(r.Values)
	snap, err := m.build(r)
	if err != nil {
		return nil, errors.New("AI settings initialization failed")
	}
	m.record = r
	m.active = snap
	return m, nil
}
func (m *Manager) Snapshot() *Snapshot { m.mu.RLock(); defer m.mu.RUnlock(); return m.active }

// VersionedSnapshot captures the configuration and its audit revision atomically.
func (m *Manager) VersionedSnapshot() (*Snapshot, int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.active, m.record.Revision
}
func (m *Manager) Public() Public { m.mu.RLock(); defer m.mu.RUnlock(); return m.publicLocked() }
func (m *Manager) publicLocked() Public {
	r := m.record
	v := r.Values
	if v.WebSearchOptions != nil {
		o := *v.WebSearchOptions
		v.WebSearchOptions = &o
	}
	v.EnabledModels = append([]string{}, v.EnabledModels...)
	source := "environment"
	if r.Revision > 0 {
		source = "platform"
	}
	return Public{HasWebSearchFallbackKey: r.WebSearchFallbackAPIKey != "", HasWebSearchKey: r.WebSearchAPIKey != "", Values: v, Revision: r.Revision, HasAPIKey: r.APIKey != "", Writable: m.store != nil, Source: source, History: append([]Audit{}, r.History...)}
}
func (m *Manager) prepare(req Update) (record, error) {
	m.mu.RLock()
	old := m.record
	m.mu.RUnlock()
	if req.Revision != old.Revision {
		return record{}, ErrConflict
	}
	if req.ComplexOutputTokens == 0 {
		req.ComplexOutputTokens = max(old.ComplexOutputTokens, req.MaxOutputTokens)
	}
	if req.ContextWindow == 0 {
		req.ContextWindow = old.ContextWindow
	}
	if req.MaxInputChars == 0 {
		req.MaxInputChars = old.MaxInputChars
	}
	if req.TaskTimeoutSeconds == 0 {
		req.TaskTimeoutSeconds = old.TaskTimeoutSeconds
	}
	req.Values = normalized(req.Values)
	req.BaseURL = strings.TrimSpace(req.BaseURL)
	base, err := normalizeURL(req.BaseURL)
	if err != nil {
		return record{}, err
	}
	req.BaseURL = base
	if req.Provider != "deepseek" && req.Provider != "openai_compatible" {
		return record{}, errors.New("请选择 DeepSeek 或 OpenAI 兼容接口")
	}
	u, _ := url.Parse(base)
	if req.Provider == "deepseek" && (u.Hostname() != "api.deepseek.com" || (u.Path != "" && u.Path != "/v1")) {
		return record{}, errors.New("DeepSeek 使用官方接口地址")
	}
	if !validModel.MatchString(req.DefaultModel) || (req.VisionModel != "" && !validModel.MatchString(req.VisionModel)) {
		return record{}, errors.New("请填写有效的默认模型和图片模型")
	}
	if req.MaxOutputTokens < 64 || req.MaxOutputTokens > 32768 {
		return record{}, errors.New("最大输出 Token 应为 64–32768")
	}
	if req.ComplexOutputTokens < req.MaxOutputTokens || req.ComplexOutputTokens > 32768 || req.ContextWindow < 32768 || req.ContextWindow > 1048576 || req.ContextWindow <= req.ComplexOutputTokens+8192 || req.MaxInputChars < 2000 || req.MaxInputChars > 100000 || req.TaskTimeoutSeconds < 60 || req.TaskTimeoutSeconds > 600 {
		return record{}, errors.New("请核对复杂输出、上下文、输入字符和任务时限；上下文必须预留工具与输出空间")
	}
	if req.ReasoningEffort != "low" && req.ReasoningEffort != "medium" && req.ReasoningEffort != "high" {
		return record{}, errors.New("推理强度必须为 low、medium 或 high")
	}
	if len(req.EnabledModels) > 100 {
		return record{}, errors.New("可选模型数量过多")
	}
	seen := map[string]bool{}
	enabled := []string{}
	for _, id := range req.EnabledModels {
		if !validModel.MatchString(id) || seen[id] {
			return record{}, errors.New("可选模型列表无效")
		}
		seen[id] = true
		enabled = append(enabled, id)
	}
	if len(enabled) > 0 && (!seen[req.DefaultModel] || (req.VisionModel != "" && !seen[req.VisionModel])) {
		return record{}, errors.New("默认模型和图片模型必须包含在开放模型中")
	}
	req.EnabledModels = enabled
	key := strings.TrimSpace(req.APIKey)
	if key == "" {
		oldURL, _ := normalizeURL(old.BaseURL)
		if base != oldURL {
			return record{}, errors.New("变更接口地址时必须填写新密钥")
		}
		key = old.APIKey
	}
	if key == "" || len(key) > 4096 || strings.ContainsAny(key, "\r\n\t ") {
		return record{}, errors.New("请填写有效的 API Key")
	}
	if req.WebSearchProvider == "" {
		req.WebSearchProvider = old.WebSearchProvider
	}
	if req.WebSearchProvider != "" && req.WebSearchProvider != "disabled" && !websearch.Supported(req.WebSearchProvider) {
		return record{}, errors.New("请选择停用、博查、腾讯云、Tavily 或 Brave Search")
	}
	searchKey := strings.TrimSpace(req.WebSearchAPIKey)
	if searchKey == "" && !req.ClearWebSearchKey {
		searchKey = old.WebSearchAPIKey
		if req.WebSearchProvider != old.WebSearchProvider && req.WebSearchProvider != "disabled" {
			searchKey = ""
		}
	}
	if req.ClearWebSearchKey {
		searchKey = ""
	}
	if len(searchKey) > 4096 || strings.ContainsAny(searchKey, "\r\n\t ") {
		return record{}, errors.New("搜索密钥格式无效")
	}
	if req.WebSearchFallbackProvider == "" {
		req.WebSearchFallbackProvider = old.WebSearchFallbackProvider
	}
	if req.WebSearchFallbackProvider != "" && req.WebSearchFallbackProvider != "disabled" && !websearch.Supported(req.WebSearchFallbackProvider) {
		return record{}, errors.New("备用搜索服务无效")
	}
	if websearch.Supported(req.WebSearchFallbackProvider) && req.WebSearchFallbackProvider == req.WebSearchProvider {
		return record{}, errors.New("主用和备用搜索服务不能相同")
	}
	fallbackKey := strings.TrimSpace(req.WebSearchFallbackAPIKey)
	if fallbackKey == "" && !req.ClearWebSearchFallbackKey && (req.WebSearchFallbackProvider == old.WebSearchFallbackProvider || req.WebSearchFallbackProvider == "disabled") {
		fallbackKey = old.WebSearchFallbackAPIKey
	}
	if req.ClearWebSearchFallbackKey {
		fallbackKey = ""
	}
	if len(fallbackKey) > 4096 || strings.ContainsAny(fallbackKey, "\r\n\t ") {
		return record{}, errors.New("备用搜索密钥格式无效")
	}
	if req.WebSearchOptions == nil {
		req.WebSearchOptions = old.WebSearchOptions
	}
	options := websearch.Options{}
	if req.WebSearchOptions != nil {
		options = *req.WebSearchOptions
	}
	if err := options.Validate(); err != nil {
		return record{}, err
	}
	req.WebSearchOptions = &options
	return record{Values: req.Values, APIKey: key, WebSearchAPIKey: searchKey, WebSearchFallbackAPIKey: fallbackKey, Revision: old.Revision}, nil
}
func (m *Manager) buildSnapshot(r record) (*Snapshot, error) {
	c := m.original
	c.WebSearch.Provider = r.WebSearchProvider
	c.WebSearch.APIKey = r.WebSearchAPIKey
	c.WebSearch.FallbackProvider = r.WebSearchFallbackProvider
	c.WebSearch.FallbackAPIKey = r.WebSearchFallbackAPIKey
	if r.WebSearchOptions != nil {
		c.WebSearch.Options = *r.WebSearchOptions
	}
	c.BaseURL = r.BaseURL
	c.APIKey = r.APIKey
	c.Model = r.DefaultModel
	c.VisionModel = r.VisionModel
	c.ThinkingEnabled = r.ThinkingEnabled
	c.ReasoningEffort = r.ReasoningEffort
	c.MaxOutputTokens = r.MaxOutputTokens
	c.ComplexOutputTokens = r.ComplexOutputTokens
	c.ContextWindow = r.ContextWindow
	c.MaxInputChars = r.MaxInputChars
	c.TaskTimeoutSeconds = r.TaskTimeoutSeconds
	c.Provider = "openai_compatible"
	if r.Provider == "mock" {
		c.Provider = "mock"
	}
	p, err := provider.New(provider.Config{Provider: c.Provider, BaseURL: c.BaseURL, APIKey: c.APIKey, Model: c.Model, VisionModel: c.VisionModel})
	if err != nil {
		return nil, err
	}
	if compatible, ok := p.(*provider.OpenAICompatible); ok {
		compatible.SetHTTPClient(secureClient())
		if r.Provider != "deepseek" {
			compatible.UseStandardParameters()
		}
	}
	if r.Provider != "deepseek" {
		c.DeepSeekPricing.Enabled = false
		c.ModelPricing = nil
		c.InputCostPerMillion = 0
		c.OutputCostPerMillion = 0
	}
	return &Snapshot{Config: c, Provider: p, Models: provider.NewCatalogWithAllowed(p, r.EnabledModels)}, nil
}
func (m *Manager) check(ctx context.Context, r record, strict bool) (*Snapshot, provider.ModelList, error) {
	r.Values = normalized(r.Values)
	snap, err := m.build(r)
	if err != nil {
		return nil, provider.ModelList{}, errors.New("无法创建模型连接")
	}
	// Discovery is unfiltered, so administrators can see all upstream models.
	list, err := provider.NewCatalog(snap.Provider).List(ctx)
	if err != nil {
		return nil, list, errors.New("连接测试失败，请检查地址、密钥和服务状态")
	}
	if strict {
		available := map[string]provider.ModelOption{}
		for _, row := range list.List {
			available[row.ID] = row
		}
		if _, ok := available[r.DefaultModel]; !ok {
			return nil, list, errors.New("默认模型不在供应商可用列表中")
		}
		if r.VisionModel != "" {
			v, ok := available[r.VisionModel]
			if !ok || !v.SupportsVision {
				return nil, list, errors.New("图片模型不可用或不支持图片")
			}
		}
		for _, id := range r.EnabledModels {
			if _, ok := available[id]; !ok {
				return nil, list, errors.New("开放模型包含供应商不可用的选项")
			}
		}
	}
	return snap, list, nil
}
func (m *Manager) Test(ctx context.Context, req Update) (provider.ModelList, error) {
	r, err := m.prepare(req)
	if err != nil {
		return provider.ModelList{}, err
	}
	_, list, err := m.check(ctx, r, false)
	return list, err
}
func (m *Manager) Save(ctx context.Context, req Update, actor string) (Public, error) {
	if m.store == nil {
		return Public{}, errors.New("服务器尚未启用持久化配置")
	}
	r, err := m.prepare(req)
	if err != nil {
		return Public{}, err
	}
	snap, _, err := m.check(ctx, r, true)
	if err != nil {
		return Public{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.record.Revision != req.Revision {
		return Public{}, ErrConflict
	}
	fields := []string{}
	before, after := reflect.ValueOf(m.record.Values), reflect.ValueOf(r.Values)
	typ := before.Type()
	for i := 0; i < before.NumField(); i++ {
		if !reflect.DeepEqual(before.Field(i).Interface(), after.Field(i).Interface()) {
			fields = append(fields, typ.Field(i).Tag.Get("json"))
		}
	}
	if m.record.APIKey != r.APIKey {
		fields = append(fields, "apiKey")
	}
	if m.record.WebSearchAPIKey != r.WebSearchAPIKey {
		fields = append(fields, "webSearchApiKey")
	}
	if m.record.WebSearchFallbackAPIKey != r.WebSearchFallbackAPIKey {
		fields = append(fields, "webSearchFallbackApiKey")
	}
	r.Revision++
	r.History = append([]Audit{{Revision: r.Revision, Actor: actor, At: time.Now().UTC().Format(time.RFC3339), Fields: fields}}, m.record.History...)
	if len(r.History) > 50 {
		r.History = r.History[:50]
	}
	if err = m.store.write(r, req.Revision); err != nil {
		return Public{}, errors.New("保存失败：" + err.Error())
	}
	m.record = r
	m.active = snap
	return m.publicLocked(), nil
}

// Older records/clients omitted these fields. Fill them without overwriting an
// explicitly stored output budget; production upgrades still use the audited API.
func normalized(v Values) Values {
	if v.ComplexOutputTokens == 0 {
		v.ComplexOutputTokens = max(16384, v.MaxOutputTokens)
	}
	if v.ContextWindow == 0 {
		v.ContextWindow = 1048576
	}
	if v.MaxInputChars == 0 {
		v.MaxInputChars = 20000
	}
	if v.TaskTimeoutSeconds == 0 {
		v.TaskTimeoutSeconds = 180
	}
	return v
}

// TestWebSearch uses a fixed public query and never returns credentials.
func (m *Manager) TestWebSearch(ctx context.Context, req Update) (string, error) {
	r, e := m.prepare(req)
	if e != nil {
		return "", e
	}
	c := m.original.WebSearch
	c.Provider = r.WebSearchProvider
	c.APIKey = r.WebSearchAPIKey
	if r.WebSearchOptions != nil {
		c.Options = *r.WebSearchOptions
	}
	// Test exactly the selected credential, never hide a primary failure behind fallback.
	c.FallbackProvider = ""
	c.FallbackAPIKey = ""
	if req.WebSearchTestTarget == "fallback" {
		c.Provider = r.WebSearchFallbackProvider
		c.APIKey = r.WebSearchFallbackAPIKey
	} else if req.WebSearchTestTarget != "" && req.WebSearchTestTarget != "primary" {
		return "", errors.New("测试目标无效")
	}
	return c.Search(ctx, "中国 国家图书馆 古籍")
}
