package logic

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/askxuan/ai-service/internal/askagent"
	"github.com/askxuan/ai-service/internal/einopoc"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/reportdoc"
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
// one bounded total deadline; nested writers must not expire before their parent.
func reportExecutionTimeout(stageSeconds int) time.Duration {
	return time.Duration(min(max(60, stageSeconds), 150)) * 4 * time.Second
}

func recordReportExecutionFailure(doc *reportdoc.Document, phase string, out askagent.Outcome, err error) {
	reason := "execution_failed"
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		reason = "deadline"
	case errors.Is(err, einopoc.ErrBudget):
		reason = "execution_budget"
	case errors.Is(err, askagent.ErrReasoningBudget):
		reason = "reasoning_budget"
	}
	// Keep classifications and counters only; provider errors can contain secrets.
	raw, _ := json.Marshal(map[string]any{"version": 2, "status": "failed", "phase": phase, "reason": reason, "modelCalls": out.ModelCalls, "agentToolCalls": out.ToolCalls})
	doc.Evidence = append(doc.Evidence, reportdoc.Evidence{Tool: "report_calibration", Text: string(raw)})
}
