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

func TestModelSpecificPricingDoesNotUseFlashRateForPro(t *testing.T) {
	conf := config.AIConf{ModelPricing: map[string]config.DeepSeekPricingConf{
		"deepseek-flash":  {Enabled: true, CacheHitOffPeakPerMillion: .003, CacheMissOffPeakPerMillion: .15, OutputOffPeakPerMillion: .6, PeakMultiplier: 2},
		"deepseek-v4-pro": {Enabled: true, CacheHitOffPeakPerMillion: .022, CacheMissOffPeakPerMillion: .66, OutputOffPeakPerMillion: 1.98, PeakMultiplier: 2},
	}}
	at := time.Date(2026, 9, 12, 0, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		id   string
		want int64
	}{{"deepseek-flash", 675}, {"deepseek-v4.1-flash", 675}, {"deepseek-v4-pro", 2310}} {
		resp := provider.Response{Model: tc.id, PromptTokens: 500, CompletionTokens: 1000}
		if got := calculateCostMicros(resp, conf, at); got != tc.want {
			t.Errorf("%s cost %d, want %d", tc.id, got, tc.want)
		}
	}
}
