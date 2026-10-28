package logic

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestFailedReportSavesChargedUsageWithoutPublishingDraft(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		event, _ := json.Marshal(map[string]any{
			"id": "failed-report", "model": "fixture-model",
			"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "content": "PRIVATE DRAFT AND REVIEW DETAIL: not report JSON"}, "finish_reason": "stop"}},
			"usage":   map[string]int{"prompt_tokens": 17, "completion_tokens": 9},
		})
		fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", event)
	}))
	defer server.Close()
	p := provider.NewOpenAICompatible(server.URL, "fixture", "fixture-model", "")
	p.SetHTTPClient(server.Client())
	p.UseStandardParameters()
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	row := sqlmock.NewRows(strings.Split(reportCols, ",")).AddRow("", "", 1, "ARfailure", "100", "fixture-report", "测试报告", "1", "PRIVATE USER QUESTION", "{}", `["资料","分析","行动"]`, 990, 0, "generating", "", "", "", 0, "2026-10-28")
	mock.ExpectQuery(`SELECT ` + reportSelectCols + ` FROM ai_report WHERE id=?`).WithArgs(int64(1)).WillReturnRows(row)
	for _, stage := range []string{"checking", "calculating", "writing"} {
		mock.ExpectExec(`UPDATE ai_report SET generation_stage=? WHERE id=? AND status='generating'`).WithArgs(stage, int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	}
	// Only audit counts may survive this malformed draft; the exact arguments
	// exclude its private text from both document_json and the public error.
	mock.ExpectExec(`UPDATE ai_report SET status='failed',generation_stage='failed',document_json=?,provider=?,model=?,prompt_tokens=?,completion_tokens=?,cost_micros=?,error_message='报告暂未生成成功，请免费重试；尚未扣款' WHERE id=? AND status='generating'`).
		WithArgs(`{"version":1,"runtime":"harness","modelCalls":1,"toolCalls":0,"blocks":[]}`, p.Name(), "fixture-model", 17, 9, int64(61), int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	s := &svc.ServiceContext{DB: sqlx.NewSqlConnFromDB(db), SkillModel: reportSkills{&model.AISkill{Code: "fixture-report", Status: model.SkillStatusEnabled, InputSchema: `{"fields":[]}`, ToolConfig: `{"enabled":false}`}}, Guard: agent.NewGuard(20000, nil), Provider: p}
	s.AIConfig.ContextWindow = 1048576
	s.AIConfig.InputCostPerMillion = 2
	s.AIConfig.OutputCostPerMillion = 3
	generateReport(s, 1)
	if calls.Load() != 1 {
		t.Fatalf("expected one charged malformed response, got %d calls", calls.Load())
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
