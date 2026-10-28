package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/askxuan/ai-service/internal/askagent"
	"github.com/askxuan/ai-service/internal/einopoc"
	"github.com/askxuan/ai-service/internal/reportdoc"
)

func TestReportExecutionFailureKeepsOnlySafeClassificationAndCounters(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{{fmt.Errorf("wrapped: %w", context.DeadlineExceeded), "deadline"}, {einopoc.ErrBudget, "execution_budget"}, {askagent.ErrReasoningBudget, "reasoning_budget"}, {errors.New("Authorization Bearer PRIVATE_KEY https://private.invalid/request"), "execution_failed"}} {
		doc := reportdoc.New()
		recordReportExecutionFailure(&doc, "writer", askagent.Outcome{ModelCalls: 3, ToolCalls: 4}, tc.err)
		audit := reportFailureAudit(doc)
		var got struct {
			Reason         string
			Phase          string
			ModelCalls     int
			AgentToolCalls int
		}
		if len(audit.Evidence) != 1 || json.Unmarshal([]byte(audit.Evidence[0].Text), &got) != nil || got.Reason != tc.want || got.Phase != "writer" || got.ModelCalls != 3 || got.AgentToolCalls != 4 {
			t.Fatalf("failure classification lost: %+v", got)
		}
		raw, _ := json.Marshal(audit)
		if strings.Contains(string(raw), "PRIVATE_KEY") || strings.Contains(string(raw), "private.invalid") || strings.Contains(string(raw), "Authorization") {
			t.Fatal("provider error leaked into persisted report audit")
		}
	}
}
