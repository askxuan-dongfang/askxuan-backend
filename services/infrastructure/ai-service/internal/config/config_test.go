package config

import (
	"os"
	"strings"
	"testing"

	"github.com/askxuan/ai-service/internal/websearch"
	"github.com/zeromicro/go-zero/core/conf"
)

func TestLegacyYAMLLoadsBeforeRuntimeDefaults(t *testing.T) {
	data, err := os.ReadFile("../../etc/ai.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, "ComplexOutputTokens:") || strings.Contains(line, "ContextWindow:") || strings.Contains(line, "TaskTimeoutSeconds:") {
			continue
		}
		lines = append(lines, line)
	}
	var cfg Config
	if err := conf.LoadFromYamlBytes([]byte(strings.Join(lines, "\n")), &cfg); err != nil {
		t.Fatalf("existing mounted YAML must load without new fields: %v", err)
	}
	for _, key := range []string{"AI_COMPLEX_OUTPUT_TOKENS", "AI_CONTEXT_WINDOW", "AI_TASK_TIMEOUT_SECONDS"} {
		t.Setenv(key, "")
	}
	runtime := cfg.AI.Runtime()
	if runtime.ComplexOutputTokens != 16384 || runtime.ContextWindow != 1048576 || runtime.TaskTimeoutSeconds != 180 {
		t.Fatalf("unexpected defaults: complex=%d context=%d timeout=%d", runtime.ComplexOutputTokens, runtime.ContextWindow, runtime.TaskTimeoutSeconds)
	}
}

func TestLegacyAndPartialSearchYAML(t *testing.T) {
	for _, raw := range []string{"WebSearch:\n  Provider: tavily\n", "WebSearch:\n  Provider: bocha\n  Options:\n    MaxSearches: 2\n"} {
		var cfg struct{ WebSearch websearch.Config }
		if err := conf.LoadFromYamlBytes([]byte(raw), &cfg); err != nil {
			t.Fatal("search YAML compatibility", err)
		}
		if !websearch.Supported(cfg.WebSearch.Provider) {
			t.Fatal("provider lost")
		}
	}
}
