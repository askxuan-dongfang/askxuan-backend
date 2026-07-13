package provider

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type Request struct {
	SystemPrompt string
	Messages     []Message
}
type Response struct {
	Content string
	Tokens  int
}

type Provider interface {
	Name() string
	Complete(ctx context.Context, req Request) (*Response, error)
}

type Config struct{ Provider, BaseURL, APIKey, Model string }

func ConfigFromEnvironment(fallback Config) Config {
	if v := os.Getenv("AI_PROVIDER"); v != "" {
		fallback.Provider = v
	}
	if v := os.Getenv("AI_BASE_URL"); v != "" {
		fallback.BaseURL = v
	}
	if v := os.Getenv("AI_API_KEY"); v != "" {
		fallback.APIKey = v
	}
	if v := os.Getenv("AI_MODEL"); v != "" {
		fallback.Model = v
	}
	if fallback.Provider == "" {
		fallback.Provider = "mock"
	}
	return fallback
}

func New(cfg Config) (Provider, error) {
	switch strings.ToLower(cfg.Provider) {
	case "mock":
		return Mock{}, nil
	case "openai_compatible":
		if cfg.BaseURL == "" || cfg.APIKey == "" || cfg.Model == "" {
			return nil, fmt.Errorf("openai_compatible requires AI_BASE_URL, AI_API_KEY and AI_MODEL")
		}
		return NewOpenAICompatible(cfg.BaseURL, cfg.APIKey, cfg.Model), nil
	default:
		return nil, fmt.Errorf("unsupported AI_PROVIDER %q", cfg.Provider)
	}
}

type Mock struct{}

func (Mock) Name() string { return "mock" }
func (Mock) Complete(_ context.Context, req Request) (*Response, error) {
	question := "你的问题"
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role == "user" {
			question = req.Messages[i].Content
			break
		}
	}
	return &Response{Content: "[本地开发模拟] 已收到：" + question + "。请结合实际情况理性判断；生产环境配置 openai_compatible Provider 后会返回模型解读。"}, nil
}
