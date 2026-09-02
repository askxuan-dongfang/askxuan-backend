package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/zeromicro/go-zero/rest"
)

// MySQLConf MySQL 连接配置
type MySQLConf struct {
	DataSource string
}

// Config ai 服务配置
type Config struct {
	rest.RestConf
	MySQL MySQLConf
	AI    AIConf
}

type AIConf struct {
	Provider             string
	BaseURL              string
	APIKey               string `json:",optional"`
	Model                string
	MinuteRequestLimit   int
	DailyRequestLimit    int
	MaxInputChars        int
	MaxHistoryMessages   int
	MaxOutputTokens      int
	InputCostPerMillion  float64
	OutputCostPerMillion float64
	BlockedTerms         []string `json:",optional"`
	MCP                  MCPConf
}

type MCPConf struct {
	Enabled bool
	BaseURL string
	Timeout int
}

func (c AIConf) Runtime() AIConf {
	c.Provider = envString("AI_PROVIDER", c.Provider)
	c.BaseURL = envString("AI_BASE_URL", c.BaseURL)
	c.APIKey = envString("AI_API_KEY", c.APIKey)
	c.Model = envString("AI_MODEL", c.Model)
	c.MinuteRequestLimit = envInt("AI_MINUTE_REQUEST_LIMIT", c.MinuteRequestLimit, 12)
	c.DailyRequestLimit = envInt("AI_DAILY_REQUEST_LIMIT", c.DailyRequestLimit, 100)
	c.MaxInputChars = envInt("AI_MAX_INPUT_CHARS", c.MaxInputChars, 2000)
	c.MaxHistoryMessages = envInt("AI_MAX_HISTORY_MESSAGES", c.MaxHistoryMessages, 30)
	c.MaxOutputTokens = envInt("AI_MAX_OUTPUT_TOKENS", c.MaxOutputTokens, 2048)
	c.InputCostPerMillion = envFloat("AI_INPUT_COST_PER_MILLION", c.InputCostPerMillion)
	c.OutputCostPerMillion = envFloat("AI_OUTPUT_COST_PER_MILLION", c.OutputCostPerMillion)
	if raw := strings.TrimSpace(os.Getenv("AI_BLOCKED_TERMS")); raw != "" {
		c.BlockedTerms = splitTerms(raw)
	}
	c.MCP.Enabled = envBool("AI_MCP_ENABLED", c.MCP.Enabled)
	c.MCP.BaseURL = envString("AI_MCP_BASE_URL", c.MCP.BaseURL)
	c.MCP.Timeout = envInt("AI_MCP_TIMEOUT_SECONDS", c.MCP.Timeout, 10)
	return c
}

func envString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, value, fallback int) int {
	if raw := strings.TrimSpace(os.Getenv(key)); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			return parsed
		}
	}
	if value > 0 {
		return value
	}
	return fallback
}

func envFloat(key string, fallback float64) float64 {
	if raw := strings.TrimSpace(os.Getenv(key)); raw != "" {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil && parsed >= 0 {
			return parsed
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if raw := strings.TrimSpace(os.Getenv(key)); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func splitTerms(raw string) []string {
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if term := strings.TrimSpace(part); term != "" {
			result = append(result, term)
		}
	}
	return result
}
