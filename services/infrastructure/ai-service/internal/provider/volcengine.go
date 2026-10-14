package provider

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
)

const VolcengineBaseURL = "https://ark.cn-beijing.volces.com/api/v3"

var arkHost = regexp.MustCompile(`^ark\.[a-z0-9-]+\.volces\.com$`)

// IsVolcengineURL identifies the Ark model service, excluding control-plane and
// subscription endpoints, which use different credentials and model contracts.
func IsVolcengineURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && arkHost.MatchString(strings.ToLower(u.Hostname())) &&
		(u.Port() == "" || u.Port() == "443") && u.User == nil && u.RawQuery == "" && u.Fragment == "" &&
		strings.TrimRight(u.Path, "/") == "/api/v3"
}

func newVolcengine(cfg Config) *OpenAICompatible {
	p := NewOpenAICompatible(cfg.BaseURL, cfg.APIKey, cfg.Model, cfg.VisionModel)
	p.configuredModels = []ModelOption{}
	seen := map[string]bool{}
	ids := append([]string{cfg.Model, cfg.VisionModel}, cfg.EnabledModels...)
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		option := ModelOption{ID: id, Name: id, Description: "火山方舟 · 文字对话", SupportsVision: id == cfg.VisionModel}
		if option.SupportsVision {
			option.Description = "火山方舟 · 文字与图片"
		}
		p.configuredModels = append(p.configuredModels, option)
	}
	sort.Slice(p.configuredModels, func(i, j int) bool { return p.configuredModels[i].ID < p.configuredModels[j].ID })
	return p
}
