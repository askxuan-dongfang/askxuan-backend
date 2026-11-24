package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrModelUnavailable = errors.New("所选模型已不可用，请刷新模型列表")
var ErrModelNeedsVision = errors.New("当前会话含图片，请选择支持图片的模型，或新建文字对话")
var modelID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:-]{0,99}$`)

const DeepSeekBaseURL = "https://api.deepseek.com"

type ModelOption struct {
	ContextWindow   int `json:"contextWindow"`
	MaxOutputTokens int `json:"maxOutputTokens"`

	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	SupportsVision bool   `json:"supportsVision"`
}
type ModelList struct {
	List               []ModelOption `json:"list"`
	DefaultModel       string        `json:"defaultModel"`
	DefaultVisionModel string        `json:"defaultVisionModel,omitempty"`
	Stale              bool          `json:"stale"`
}
type Catalog struct {
	provider           Provider
	allowed            map[string]bool
	mu                 sync.Mutex
	list               []ModelOption
	fetched, attempted time.Time
	lastErr            error
}

func NewCatalog(p Provider) *Catalog { return &Catalog{provider: p} }
func NewCatalogWithAllowed(p Provider, ids []string) *Catalog {
	c := NewCatalog(p)
	if len(ids) > 0 {
		c.allowed = make(map[string]bool, len(ids))
		for _, id := range ids {
			c.allowed[id] = true
		}
	}
	return c
}

func DiscoverDeepSeek(ctx context.Context, client *http.Client, key string) ([]ModelOption, error) {
	p := NewOpenAICompatible(DeepSeekBaseURL, key, "", "")
	p.SetHTTPClient(client)
	return p.listModels(ctx)
}

// Model IDs come from the authenticated upstream catalog. Descriptions and
// capabilities are our compatibility metadata, not inferred from arbitrary IDs.
func (p *OpenAICompatible) listModels(ctx context.Context) ([]ModelOption, error) {
	// Ark Endpoint IDs are account resources, not a discoverable /models catalog.
	// Settings probes these explicit IDs before publishing the immutable snapshot.
	if p.configuredModels != nil {
		return append([]ModelOption{}, p.configuredModels...), nil
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.baseURL+"/models", nil)
	if err != nil {
		return nil, errors.New("模型列表地址无效")
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	response, err := p.client.Do(req)
	if err != nil {
		return nil, errors.New("模型列表暂时无法连接")
	}
	defer response.Body.Close()
	deepSeek := p.baseURL == DeepSeekBaseURL || p.baseURL == DeepSeekBaseURL+"/v1"
	if response.StatusCode != http.StatusOK {
		if deepSeek {
			return nil, &HTTPError{StatusCode: response.StatusCode}
		}
		return nil, fmt.Errorf("模型列表暂时不可用（%d）", response.StatusCode)
	}
	var body struct {
		Data []struct {
			ID               string   `json:"id"`
			Name             string   `json:"name"`
			ContextWindow    int      `json:"context_window"`
			MaxOutputTokens  int      `json:"max_output_tokens"`
			InputModalities  []string `json:"input_modalities"`
			OutputModalities []string `json:"output_modalities"`
		} `json:"data"`
	}
	if err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&body); err != nil {
		return nil, errors.New("模型列表响应无效")
	}
	seen := map[string]bool{}
	result := []ModelOption{}
	for _, row := range body.Data {
		if !modelID.MatchString(row.ID) || seen[row.ID] {
			continue
		}
		if deepSeek && ((len(row.InputModalities) > 0 && !slices.Contains(row.InputModalities, "text")) ||
			(len(row.OutputModalities) > 0 && !slices.Contains(row.OutputModalities, "text"))) {
			continue
		}
		seen[row.ID] = true
		option := ModelOption{ContextWindow: row.ContextWindow, MaxOutputTokens: row.MaxOutputTokens, ID: row.ID, Name: row.ID, Description: "文字对话", SupportsVision: row.ID == p.visionModel}
		if name := strings.TrimSpace(row.Name); deepSeek && name != "" && len(name) <= 120 {
			option.Name = name
		}
		switch row.ID {
		case "deepseek-flash", "deepseek-v4-flash", "deepseek-v4-flash-vision-exp":
			option.Name = "DeepSeek Flash"
			option.Description = "文字与图片"
			option.SupportsVision = true
		case "deepseek-v4-pro":
			option.Name = "DeepSeek V4 Pro"
			option.Description = "文字对话 · 不支持图片"
			option.SupportsVision = false
		}
		if deepSeek && len(row.InputModalities) > 0 {
			option.SupportsVision = slices.Contains(row.InputModalities, "image")
			option.Description = "文字对话"
			if option.SupportsVision {
				option.Description = "文字与图片"
			}
		}
		result = append(result, option)
	}
	if len(result) == 0 {
		return nil, errors.New("当前账号没有可用模型")
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}
func (c *Catalog) List(ctx context.Context) (ModelList, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.provider.(Mock); ok {
		return ModelList{List: []ModelOption{{ID: "mock", Name: "本地模拟", Description: "开发环境"}}, DefaultModel: "mock"}, nil
	}
	now := time.Now()
	if now.Sub(c.fetched) > 5*time.Minute && now.Sub(c.attempted) > 15*time.Second {
		c.attempted = now
		if upstream, ok := c.provider.(interface {
			listModels(context.Context) ([]ModelOption, error)
		}); ok {
			list, err := upstream.listModels(ctx)
			c.lastErr = err
			if err == nil {
				c.list = list
				c.fetched = time.Now()
			}
		} else {
			c.lastErr = errors.New("当前服务未提供模型列表")
		}
	}
	if len(c.list) == 0 || now.Sub(c.fetched) > time.Hour {
		if c.lastErr != nil {
			return ModelList{}, c.lastErr
		}
		return ModelList{}, errors.New("模型列表暂不可用")
	}
	result := ModelList{List: append([]ModelOption(nil), c.list...), Stale: c.lastErr != nil}
	if len(c.allowed) > 0 {
		result.List = []ModelOption{}
		for _, row := range c.list {
			if c.allowed[row.ID] {
				result.List = append(result.List, row)
			}
		}
		if len(result.List) == 0 {
			return ModelList{}, errors.New("管理员开放的模型暂不可用")
		}
	}
	c.setDefaults(&result)
	return result, nil
}
func (c *Catalog) Select(ctx context.Context, id string, images bool) (string, error) {
	id = strings.TrimSpace(id)
	// Existing clients omit model; preserve their configured text/image routing.
	if id == "" {
		return c.defaultFor(images), nil
	}
	list, err := c.List(ctx)
	if err != nil {
		return "", err
	}
	return list.selectModel(id, images)
}

func (c *Catalog) CustomerList(ctx context.Context, ids []string) (ModelList, error) {
	list, err := c.List(ctx)
	if err != nil {
		return list, err
	}
	if len(ids) == 0 {
		list.List = slices.DeleteFunc(list.List, func(m ModelOption) bool {
			return !c.availableToCustomerByDefault(m.ID)
		})
		if len(list.List) == 0 {
			return ModelList{}, ErrModelUnavailable
		}
		c.setDefaults(&list)
		return list, nil
	}
	allowed := make(map[string]bool, len(ids))
	for _, id := range ids {
		allowed[id] = true
	}
	options := list.List
	list.List = make([]ModelOption, 0, len(ids))
	for _, model := range options {
		if model.ID == "mock" || !allowed[model.ID] {
			continue
		}
		delete(allowed, model.ID)
		list.List = append(list.List, model)
	}
	c.setDefaults(&list)
	return list, nil
}

// Separately registered subscription models enter the automatic list only after
// the administrator assigns them to customer-facing text or image use.
func (c *Catalog) availableToCustomerByDefault(id string) bool {
	if r, ok := c.provider.(*Registry); ok {
		return !r.routes[id].ExplicitCustomerChoice || id == r.defaultModel || id == r.vision
	}
	return true
}

func (c *Catalog) SelectForCustomer(ctx context.Context, ids []string, id string, images bool) (string, error) {
	if len(ids) == 0 {
		if !c.availableToCustomerByDefault(strings.TrimSpace(id)) {
			return "", ErrModelUnavailable
		}
		return c.Select(ctx, id, images)
	}
	list, err := c.CustomerList(ctx, ids)
	if err != nil {
		return "", err
	}
	if id = strings.TrimSpace(id); id != "" {
		return list.selectModel(id, images)
	}
	if list.DefaultModel == "" {
		return "", ErrModelUnavailable
	}
	if !images {
		return list.DefaultModel, nil
	}
	if list.DefaultVisionModel == "" {
		return "", ErrModelNeedsVision
	}
	return list.DefaultVisionModel, nil
}

func (c *Catalog) setDefaults(list *ModelList) {
	chat, vision := c.provider.Model(), c.defaultFor(true)
	list.DefaultModel, list.DefaultVisionModel = "", ""
	for _, model := range list.List {
		if list.DefaultModel == "" || model.ID == chat {
			list.DefaultModel = model.ID
		}
		if model.SupportsVision && (list.DefaultVisionModel == "" || model.ID == vision) {
			list.DefaultVisionModel = model.ID
		}
	}
}

func (c *Catalog) defaultFor(images bool) string {
	request := Request{}
	if images {
		request.Messages = []Message{{ImageDataURLs: []string{"present"}}}
	}
	return c.provider.ModelFor(request)
}

func (l ModelList) selectModel(id string, images bool) (string, error) {
	for _, row := range l.List {
		if row.ID == id {
			if images && !row.SupportsVision {
				return "", ErrModelNeedsVision
			}
			return id, nil
		}
	}
	return "", ErrModelUnavailable
}

// Limits fails closed if discovery fails; unknown metadata uses a conservative
// window instead of assuming every compatible model supports DeepSeek's 1M.
func (c *Catalog) Limits(ctx context.Context, id string, platform, output int) (int, int, error) {
	list, err := c.List(ctx)
	if err != nil {
		return 0, 0, err
	}
	for _, m := range list.List {
		if m.ID != id {
			continue
		}
		window := m.ContextWindow
		if window <= 0 {
			window = 32768
		}
		window = min(window, 1048576)
		if platform > 0 {
			window = min(window, platform)
		}
		if m.MaxOutputTokens > 0 {
			output = min(output, m.MaxOutputTokens)
		}
		output = min(output, window/2)
		if output < 64 {
			return 0, 0, errors.New("model token limits unavailable")
		}
		return window, output, nil
	}
	return 0, 0, ErrModelUnavailable
}
