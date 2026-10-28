package logic

import (
	"context"
	"strings"
	"time"

	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/svc"
)

func selectReportModel(ctx context.Context, s *svc.ServiceContext, topic string) (string, error) {
	if topic == "face_palm" {
		return reportVisionModel(ctx, s)
	}
	if selected := strings.TrimSpace(s.AIConfig.ReportModel); selected != "" {
		if s.Models == nil {
			return "", provider.ErrModelUnavailable
		}
		return s.Models.Select(ctx, selected, false)
	}
	return s.Provider.ModelFor(provider.Request{Model: s.AgentDefaultModel}), nil
}

// A report may write, review, repair once, and review again. Give those stages
// one bounded total deadline while retaining each harness's configured limit.
func reportExecutionTimeout(stageSeconds int) time.Duration {
	return time.Duration(min(max(60, stageSeconds), 150)) * 4 * time.Second
}
