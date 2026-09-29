package config

import (
	"os"
	"strings"
	"testing"

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
