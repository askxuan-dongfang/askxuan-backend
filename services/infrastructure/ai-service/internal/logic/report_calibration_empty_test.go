package logic

import (
	"fmt"
	"strings"
	"testing"

	"github.com/askxuan/ai-service/internal/provider"
)

func TestReportCalibrationEmptyLogUsesOnlyAllowlistedMetadata(t *testing.T) {
	const privateContent = "PRIVATE BODY\nPRIVATE REASONING\nHTTP 502 https://private.invalid/v1?token=PRIVATE_TOKEN"
	const privateModel = "PRIVATE_MODEL"
	type logCase struct {
		name   string
		review *provider.Response
		finish string
	}
	tests := []logCase{
		{name: "nil", finish: "unknown"},
	}
	for _, finish := range []string{"stop", "length", "content_filter", "tool_calls", "function_call"} {
		tests = append(tests, logCase{name: finish, review: &provider.Response{FinishReason: finish}, finish: finish})
	}
	for _, finish := range []string{"", "PRIVATE_FINISH", "stop\nreport_id=999 PRIVATE_FINISH", " stop", "STOP"} {
		tests = append(tests, logCase{name: "unknown " + finish, review: &provider.Response{FinishReason: finish}, finish: "unknown"})
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			prompt, completion := 0, 0
			var before provider.Response
			if test.review != nil {
				test.review.Content, test.review.Model = privateContent, privateModel
				test.review.PromptTokens, test.review.CompletionTokens = 17, 23
				test.review.PromptCacheHitTokens, test.review.PromptCacheMissTokens = 11, 6
				test.review.ReasoningTokens = 19 // Included in completion usage; never add it again.
				prompt, completion, before = 17, 23, *test.review
			}
			got := reportCalibrationEmptyLog(194, test.review, 8192)
			want := fmt.Sprintf("report calibration empty response report_id=194 finish=%s prompt_tokens=%d completion_tokens=%d max_tokens=8192 empty_content=true", test.finish, prompt, completion)
			if got != want {
				t.Fatalf("safe log format mismatch: got %q, want %q", got, want)
			}
			for _, forbidden := range []string{"\n", "PRIVATE", "HTTP", "https://", "reasoning_tokens", "cache_hit", "cache_miss"} {
				if strings.Contains(got, forbidden) {
					t.Fatalf("safe log leaked %q", forbidden)
				}
			}
			if test.review != nil && *test.review != before {
				t.Fatal("safe log changed provider response or usage")
			}
		})
	}
}
