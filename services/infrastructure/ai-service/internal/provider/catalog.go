package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrModelUnavailable = errors.New("所选模型已不可用，请刷新模型列表")
var ErrModelNeedsVision = errors.New("当前会话含图片，请选择支持图片的模型，或新建文字对话")
var modelID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:-]{0,99}$`)

type ModelOption struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	SupportsVision bool   `json:"supportsVision"`
}
type ModelList struct {
	List         []ModelOption `json:"list"`
	DefaultModel string        `json:"defaultModel"`
	Stale        bool          `json:"stale"`
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

// Model IDs come from the authenticated upstream catalog. Descriptions and
// capabilities are our compatibility metadata, not inferred from arbitrary IDs.
func (p *OpenAICompatible) listModels(ctx context.Context) ([]ModelOption, error) {
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
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("模型列表暂时不可用（%d）", response.StatusCode)
	}
	var body struct {
		Data []struct {
			ID string `json:"id"`
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
		seen[row.ID] = true
		option := ModelOption{ID: row.ID, Name: row.ID, Description: "文字对话", SupportsVision: row.ID == p.visionModel}
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
		if upstream, ok := c.provider.(*OpenAICompatible); ok {
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
	for _, row := range result.List {
		if row.ID == c.provider.Model() {
			result.DefaultModel = row.ID
		}
	}
	if result.DefaultModel == "" {
		result.DefaultModel = result.List[0].ID
	}
	return result, nil
}
func (c *Catalog) Select(ctx context.Context, id string, images bool) (string, error) {
	id = strings.TrimSpace(id)
	// Existing clients omit model; preserve their configured text/image routing.
	if id == "" {
		request := Request{}
		if images {
			request.Messages = []Message{{ImageDataURLs: []string{"present"}}}
		}
		return c.provider.ModelFor(request), nil
	}
	list, err := c.List(ctx)
	if err != nil {
		return "", err
	}
	for _, row := range list.List {
		if row.ID == id {
			if images && !row.SupportsVision {
				return "", ErrModelNeedsVision
			}
			return id, nil
		}
	}
	return "", ErrModelUnavailable
}
