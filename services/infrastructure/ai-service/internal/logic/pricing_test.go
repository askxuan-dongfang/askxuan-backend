package logic

import (
	"testing"
	"time"

	"github.com/askxuan/ai-service/internal/config"
	"github.com/askxuan/ai-service/internal/provider"
)

func TestCalculateDeepSeekCostUsesCacheAndPeakMultiplier(t *testing.T) {
	conf := config.AIConf{DeepSeekPricing: config.DeepSeekPricingConf{Enabled: true, CacheHitOffPeakPerMillion: 0.007, CacheMissOffPeakPerMillion: 0.22, OutputOffPeakPerMillion: 0.66, PeakMultiplier: 2}}
	resp := provider.Response{Model: "deepseek-v4-flash", PromptTokens: 1500, PromptCacheHitTokens: 1000, PromptCacheMissTokens: 500, CompletionTokens: 1000}
	offPeak := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	if got := calculateCostMicros(resp, conf, offPeak); got != 777 {
		t.Fatalf("off-peak cost=%d, want 777", got)
	}
	peak := time.Date(2026, 9, 2, 6, 30, 0, 0, time.UTC)
	if got := calculateCostMicros(resp, conf, peak); got != 1554 {
		t.Fatalf("peak cost=%d, want 1554", got)
	}
}
