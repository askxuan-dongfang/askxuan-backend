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
)

var ErrConflict = errors.New("配置已被其他管理员更新，请重新加载后再保存")
var validModel = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:-]{0,99}$`)

type Values struct {
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
	Values
	Revision  int64   `json:"revision"`
	HasAPIKey bool    `json:"hasApiKey"`
	Writable  bool    `json:"writable"`
	Source    string  `json:"source"`
	History   []Audit `json:"history"`
}
type record struct {
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
	r := record{Values: Values{Provider: kind, BaseURL: initial.BaseURL, DefaultModel: initial.Model, VisionModel: initial.VisionModel, ThinkingEnabled: initial.ThinkingEnabled, ReasoningEffort: initial.ReasoningEffort, MaxOutputTokens: initial.MaxOutputTokens}, APIKey: initial.APIKey, History: []Audit{}}
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
	snap, err := m.build(r)
	if err != nil {
		return nil, errors.New("AI settings initialization failed")
	}
	m.record = r
	m.active = snap
	return m, nil
}
func (m *Manager) Snapshot() *Snapshot { m.mu.RLock(); defer m.mu.RUnlock(); return m.active }
func (m *Manager) Public() Public      { m.mu.RLock(); defer m.mu.RUnlock(); return m.publicLocked() }
func (m *Manager) publicLocked() Public {
	r := m.record
	v := r.Values
	v.EnabledModels = append([]string{}, v.EnabledModels...)
	source := "environment"
	if r.Revision > 0 {
		source = "platform"
	}
	return Public{Values: v, Revision: r.Revision, HasAPIKey: r.APIKey != "", Writable: m.store != nil, Source: source, History: append([]Audit{}, r.History...)}
}
func (m *Manager) prepare(req Update) (record, error) {
	m.mu.RLock()
	old := m.record
	m.mu.RUnlock()
	if req.Revision != old.Revision {
		return record{}, ErrConflict
	}
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
	return record{Values: req.Values, APIKey: key, Revision: old.Revision}, nil
}
func (m *Manager) buildSnapshot(r record) (*Snapshot, error) {
	c := m.original
	c.BaseURL = r.BaseURL
	c.APIKey = r.APIKey
	c.Model = r.DefaultModel
	c.VisionModel = r.VisionModel
	c.ThinkingEnabled = r.ThinkingEnabled
	c.ReasoningEffort = r.ReasoningEffort
	c.MaxOutputTokens = r.MaxOutputTokens
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
