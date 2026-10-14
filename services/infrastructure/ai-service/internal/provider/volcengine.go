package provider

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
)

const VolcengineBaseURL = "https://ark.cn-beijing.volces.com/api/v3"
const VolcenginePlanBaseURL = "https://ark.cn-beijing.volces.com/api/plan/v3"

var arkHost = regexp.MustCompile(`^ark\.[a-z0-9-]+\.volces\.com$`)

// IsVolcengineURL accepts only the official model-service and Agent Plan roots.
func IsVolcengineURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && arkHost.MatchString(strings.ToLower(u.Hostname())) &&
		(u.Port() == "" || u.Port() == "443") && u.User == nil && u.RawQuery == "" && u.Fragment == "" &&
		(u.EscapedPath() == u.Path) && (strings.TrimRight(u.Path, "/") == "/api/v3" || strings.TrimRight(u.Path, "/") == "/api/plan/v3")
}

func IsVolcenginePlanURL(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && IsVolcengineURL(raw) && strings.TrimRight(u.Path, "/") == "/api/plan/v3"
}

// Documented Agent Plan limits, verified 2026-10-14:
// https://docs.volcengine.com/docs/ark/agent-plan-enterprise-opencode?lang=zh
// https://docs.volcengine.com/docs/ark/agent-plan-enterprise-plan-overview?lang=zh
// Use decimal limits from Ark's client examples. Floating aliases retain the
// documented 256K/32K baseline instead of assuming the current target's 1M.
// Unknown IDs and ordinary Endpoint IDs keep Catalog.Limits' safe fallback.
func volcenginePlanLimits(id string) (window, output int) {
	switch id {
	case "ark-code-latest", "glm-latest", "minimax-latest", "kimi-latest", "deepseek-latest", "kimi-k2.7-code":
		return 256000, 32000
	case "glm-5.3", "glm-5.3-flash", "minimax-m3", "kimi-k3":
		return 1024000, 128000
	case "deepseek-v4-flash", "deepseek-v4.1-flash", "deepseek-v4-pro":
		return 1024000, 384000
	case "doubao-seed-2.1-lite", "doubao-seed-2.1-pro", "doubao-seed-evolving":
		return 1024000, 256000
	case "doubao-seed-2.0-mini":
		return 256000, 128000
	case "kimi-k2.8-preview":
		return 1024000, 1024000
	default:
		return 0, 0
	}
}

func newVolcengine(cfg Config) *OpenAICompatible {
	p := NewOpenAICompatible(cfg.BaseURL, cfg.APIKey, cfg.Model, cfg.VisionModel)
	// Plan aliases may route to models that cannot disable thinking. Let the
	// selected model choose its reasoning defaults on every execution path.
	p.modelDefaultReasoning = IsVolcenginePlanURL(cfg.BaseURL)
	p.configuredModels = []ModelOption{}
	seen := map[string]bool{}
	ids := append([]string{cfg.Model, cfg.VisionModel}, cfg.EnabledModels...)
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		option := ModelOption{ID: id, Name: id, Description: "火山方舟 · 文字对话", SupportsVision: id == cfg.VisionModel}
		if IsVolcenginePlanURL(cfg.BaseURL) {
			option.ContextWindow, option.MaxOutputTokens = volcenginePlanLimits(id)
		}
		if option.SupportsVision {
			option.Description = "火山方舟 · 文字与图片"
		}
		p.configuredModels = append(p.configuredModels, option)
	}
	sort.Slice(p.configuredModels, func(i, j int) bool { return p.configuredModels[i].ID < p.configuredModels[j].ID })
	return p
}
