package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestReportModelSelectionKeepsVisionAndChatDefaults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"data":[{"id":"deepseek-flash","context_window":1048576,"max_output_tokens":32768},{"id":"deepseek-v4-pro","context_window":1048576,"max_output_tokens":32768},{"id":"not-enabled"}]}`)
	}))
	defer server.Close()
	p := provider.NewOpenAICompatible(server.URL, "fixture", "deepseek-flash", "")
	s := &svc.ServiceContext{Provider: p, Models: provider.NewCatalogWithAllowed(p, []string{"deepseek-flash", "deepseek-v4-pro"}), AgentDefaultModel: "published-chat"}
	s.AIConfig.ReportModel = "deepseek-v4-pro"
	for _, tc := range []struct{ topic, want string }{{"bazi", "deepseek-v4-pro"}, {"marriage", "deepseek-v4-pro"}, {"face_palm", "deepseek-flash"}} {
		got, err := selectReportModel(context.Background(), s, tc.topic)
		if err != nil || got != tc.want {
			t.Fatalf("topic=%s model=%s err=%v", tc.topic, got, err)
		}
	}
	s.AIConfig.ReportModel = "not-enabled"
	if _, err := selectReportModel(context.Background(), s, "bazi"); err == nil {
		t.Fatal("disabled report model silently fell back to another model")
	}
	s.AIConfig.ReportModel = ""
	if got, err := selectReportModel(context.Background(), s, "bazi"); err != nil || got != "published-chat" {
		t.Fatal("empty report override discarded published chat selection", got, err)
	}
	if p.Model() != "deepseek-flash" || s.AgentDefaultModel != "published-chat" || s.AIConfig.ThinkingEnabled {
		t.Fatal("report selection mutated shared defaults")
	}
}

func TestReportReasoningPolicyAndExhaustionAreRequestLocal(t *testing.T) {
	for _, exhausted := range []bool{false, true} {
		t.Run(fmt.Sprintf("exhausted=%t", exhausted), func(t *testing.T) {
			var writers, reviews atomic.Int32
			body := reportBody{Summary: strings.Repeat("合成报告摘要用于校验参数。", 3), Content: "## 依据\n" + strings.Repeat("只使用本次资料，不代表真实模型效果。", 30) + "\n## 对照\n核对实际条件。\n## 行动\n记录并复盘。"}
			encoded, _ := json.Marshal(body)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/models" {
					fmt.Fprint(w, `{"data":[{"id":"deepseek-flash","context_window":1048576,"max_output_tokens":16384},{"id":"deepseek-v4-pro","context_window":1048576,"max_output_tokens":16384}]}`)
					return
				}
				var req map[string]any
				_ = json.NewDecoder(r.Body).Decode(&req)
				thinking, _ := req["thinking"].(map[string]any)
				if req["model"] != "deepseek-v4-pro" || thinking["type"] != "enabled" || req["reasoning_effort"] != "high" {
					t.Error("report writer/reviewer lost its selected reasoning policy")
				}
				if isReportCalibrationFixtureRequest(req) {
					reviews.Add(1)
					if req["max_tokens"] != float64(16384) {
						t.Error("review did not receive its bounded reasoning budget")
					}
					writeReportCalibrationFixture(w, `{"issues":[]}`)
					return
				}
				writers.Add(1)
				if req["max_tokens"] != float64(16384) {
					t.Error("writer exceeded the selected model output limit")
				}
				choice := map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "content": string(encoded)}, "finish_reason": "stop"}
				usage := map[string]any{"prompt_tokens": 13, "completion_tokens": 9, "completion_tokens_details": map[string]any{"reasoning_tokens": 5}}
				if exhausted {
					choice["delta"] = map[string]any{"role": "assistant", "reasoning_content": "PRIVATE REASONING: not a report"}
					choice["finish_reason"] = "length"
					usage["completion_tokens"] = 16384
					usage["completion_tokens_details"] = map[string]any{"reasoning_tokens": 16384}
				}
				event, _ := json.Marshal(map[string]any{"model": "deepseek-v4-pro", "choices": []any{choice}, "usage": usage})
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", event)
			}))
			defer server.Close()
			p := provider.NewOpenAICompatible(server.URL, "fixture", "deepseek-flash", "")
			db, mock, _ := sqlmock.New()
			defer db.Close()
			for _, stage := range []string{"checking", "calculating", "writing"} {
				mock.ExpectExec("UPDATE ai_report SET generation_stage").WithArgs(stage, int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			s := &svc.ServiceContext{DB: sqlx.NewSqlConnFromDB(db), Provider: p, Models: provider.NewCatalog(p), Guard: agent.NewGuard(20000, nil), SkillModel: reportSkills{&model.AISkill{Code: "fixture-topic", Status: model.SkillStatusEnabled, InputSchema: `{"fields":[]}`, ToolConfig: `{"enabled":false}`}}}
			s.AIConfig.ReportModel = "deepseek-v4-pro"
			s.AIConfig.ContextWindow = 1048576
			s.AIConfig.ComplexOutputTokens = 32768
			s.AIConfig.ReasoningEffort = "low"
			r := &Report{ID: 1, SkillCode: "fixture-topic", Question: "合成报告", InputsJSON: `{}`, ChaptersJSON: `["依据","对照","行动"]`}
			got, doc, usage, err := executeReport(context.Background(), s, r)
			if exhausted {
				if err == nil || writers.Load() != 1 || reviews.Load() != 0 || doc.ModelCalls != 1 || got.Content != "" || usage.PromptTokens != 13 || usage.CompletionTokens != 16384 || usage.ReasoningTokens != 16384 {
					t.Fatalf("reasoning exhaustion retried/published/lost usage: err=%v writers=%d reviews=%d models=%d usage=%+v", err, writers.Load(), reviews.Load(), doc.ModelCalls, usage)
				}
			} else if err != nil || got.Content != body.Content || writers.Load() != 1 || reviews.Load() != 1 || doc.ModelCalls != 2 || usage.Model != "deepseek-v4-pro" || usage.PromptTokens != 24 || usage.CompletionTokens != 16 || usage.ReasoningTokens != 8 {
				t.Fatalf("report profile/accounting incorrect: err=%v writers=%d reviews=%d models=%d usage=%+v", err, writers.Load(), reviews.Load(), doc.ModelCalls, usage)
			}
			if p.Model() != "deepseek-flash" || s.AIConfig.ThinkingEnabled || s.AIConfig.ReasoningEffort != "low" {
				t.Fatal("report reasoning policy escaped into ordinary chat configuration")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReportExecutionTimeoutIsBoundedForAllPhases(t *testing.T) {
	for _, tc := range []struct {
		seconds int
		want    time.Duration
	}{{0, 4 * time.Minute}, {60, 4 * time.Minute}, {120, 8 * time.Minute}, {180, 10 * time.Minute}, {600, 10 * time.Minute}, {1 << 30, 10 * time.Minute}} {
		if got := reportExecutionTimeout(tc.seconds); got != tc.want || got >= 15*time.Minute {
			t.Fatalf("timeout=%s for stage=%d", got, tc.seconds)
		}
	}
}

type retryUsageFixture struct {
	model.UsageModel
	calls atomic.Int32
}

func (u *retryUsageFixture) Acquire(context.Context, string, int, int) error {
	u.calls.Add(1)
	return nil
}

func TestConcurrentReportRetriesCannotReclaimAnActiveTenMinuteJob(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.MatchExpectationsInOrder(false)
	for i := 0; i < 4; i++ {
		mock.ExpectQuery(`SELECT `+reportSelectCols+` FROM ai_report WHERE id=? AND user_id=?`).WithArgs(int64(1), "100").WillReturnRows(sqlmock.NewRows(strings.Split(reportCols, ",")).AddRow("", "writing", 1, "ARactive", "100", "bazi", "fixture", "1", "synthetic", "{}", "[]", 990, 0, "generating", "", "", "", 0, "2026-10-28"))
		mock.ExpectQuery(reportCashEntitlementSQL).WithArgs("ARactive", "100", int64(990), int64(990)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	}
	for i := 0; i < 2; i++ {
		mock.ExpectExec(`UPDATE ai_report SET status='generating',version=?,error_message='',updated_at=CURRENT_TIMESTAMP WHERE id=? AND user_id=? AND (status='failed' OR (status='generating' AND updated_at<DATE_SUB(NOW(),INTERVAL 15 MINUTE)))`).WithArgs("1@a3", int64(1), "100").WillReturnResult(sqlmock.NewResult(0, 0))
	}
	usage := &retryUsageFixture{}
	s := &svc.ServiceContext{DB: sqlx.NewSqlConnFromDB(db), UsageModel: usage, AgentVersion: 3}
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := ReportRetry(context.Background(), s, "100", 1)
			if e != nil || r == nil || r.Status != "generating" {
				t.Errorf("retry changed active report: %v", e)
			}
		}()
	}
	wg.Wait()
	if usage.calls.Load() != 2 {
		t.Fatal("retry fixture did not exercise both callers")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
