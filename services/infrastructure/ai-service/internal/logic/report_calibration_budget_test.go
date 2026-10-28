package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/reportdoc"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestCalibration32KBudgetKeepsFullEvidenceAndMargin(t *testing.T) {
	in := reportCalibrationInput{
		Topic: "marriage", Question: "双方完整依据核对", Inputs: json.RawMessage(`{"you":"甲","partner":"乙"}`),
		Calculations: json.RawMessage(`[{"subject":"you","value":"甲子"},{"subject":"partner","value":"乙丑"}]`),
		Charts:       []reportdoc.Block{{Kind: "table", Title: "双方图表", Items: []reportdoc.Item{{Label: "你", Value: "甲子"}, {Label: "对方", Value: "乙丑"}}}},
		Sources:      []reportdoc.Evidence{{Tool: "search_knowledge", Text: strings.Repeat("完整片段必须保留。", 1000)}},
		Draft:        reportBody{Summary: "完整摘要", Content: strings.Repeat("完整正文。", 1000)},
	}
	req, err := reportCalibrationRequest(provider.Request{MaxTokens: 65536, ThinkingEnabled: true, ReasoningEffort: "high"}, 1048576, in)
	if err != nil || req.MaxTokens != 32768 {
		t.Fatalf("review cap=%d err=%v, want 32768", req.MaxTokens, err)
	}
	var actual reportCalibrationInput
	if json.Unmarshal([]byte(req.Messages[0].Content), &actual) != nil {
		t.Fatal("missing full calibration input")
	}
	before, _ := json.Marshal(in)
	after, _ := json.Marshal(actual)
	if string(before) != string(after) {
		t.Fatal("larger output budget omitted or changed participants, charts, sources or draft")
	}
	boundary := len(req.SystemPrompt) + len(req.Messages[0].Content) + 32768 + 4096
	if _, err := reportCalibrationRequest(req, boundary, in); err != nil {
		t.Fatal("exact full-evidence budget should fit", err)
	}
	if _, err := reportCalibrationRequest(req, boundary-1, in); err == nil {
		t.Fatal("review lost its fixed 4096-token margin")
	}
}

func TestReportCalibrationOutputBudget(t *testing.T) {
	for _, tc := range []struct {
		name                          string
		catalog                       bool
		window, modelWindow, modelCap int
		writerOutput, wantWriter      int
		wantReview, padding           int
		finish, reviewContent         string
	}{
		{name: "live_catalog_1m_393216", catalog: true, window: 1048576, modelWindow: 1048576, modelCap: 393216, wantWriter: 16384, wantReview: 32768},
		{name: "small_model_cap", catalog: true, window: 1048576, modelWindow: 1048576, modelCap: 8192, wantWriter: 8192, wantReview: 8192},
		{name: "intermediate_model_cap", catalog: true, window: 1048576, modelWindow: 1048576, modelCap: 24576, wantWriter: 16384, wantReview: 24576},
		{name: "unknown_output_cap", catalog: true, window: 1048576, modelWindow: 1048576, wantWriter: 16384, wantReview: 16384},
		{name: "unknown_metadata", catalog: true, window: 1048576, wantWriter: 16384, wantReview: 16384},
		{name: "nil_catalog_writer_32k", window: 1048576, writerOutput: 32768, wantWriter: 32768, wantReview: 16384},
		{name: "unknown_cap_writer_32k", catalog: true, window: 1048576, modelWindow: 1048576, writerOutput: 32768, wantWriter: 32768, wantReview: 16384},
		{name: "32k_context_keeps_16k", catalog: true, window: 32768, modelWindow: 1048576, modelCap: 393216, wantWriter: 16384, wantReview: 16384},
		{name: "full_draft_falls_back_to_16k", catalog: true, window: 65536, modelWindow: 1048576, modelCap: 393216, padding: 30000, wantWriter: 16384, wantReview: 16384},
		{name: "full_draft_falls_back_to_original_8k", catalog: true, window: 65536, modelWindow: 1048576, modelCap: 393216, writerOutput: 8192, padding: 30000, wantWriter: 8192, wantReview: 8192},
		{name: "length_json_is_not_passed", catalog: true, window: 1048576, modelWindow: 1048576, modelCap: 393216, wantWriter: 16384, wantReview: 32768, finish: "length", reviewContent: `{"issues":[]}`},
		{name: "empty_length_is_not_retried", catalog: true, window: 1048576, modelWindow: 1048576, modelCap: 393216, wantWriter: 16384, wantReview: 32768, finish: "length"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := reportBody{Summary: strings.Repeat("合成预算测试摘要。", 4), Content: "## 依据\n" + strings.Repeat("保留完整草稿，不代表实际模型质量。", 25) + strings.Repeat("x", tc.padding) + "\n## 对照\n按已确认资料对照。\n## 行动\n记录并复盘。"}
			rawBody, _ := json.Marshal(body)
			writers, reviews := 0, 0
			p := provider.NewOpenAICompatible("http://calibration-budget.invalid", "fixture", "fixture", "")
			p.SetHTTPClient(&http.Client{Transport: defaultContextTransport(func(r *http.Request) (*http.Response, error) {
				w := httptest.NewRecorder()
				if r.Method == http.MethodGet && r.URL.Path == "/models" {
					_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"id": "fixture", "context_window": tc.modelWindow, "max_output_tokens": tc.modelCap}}})
				} else {
					var req map[string]any
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						return nil, err
					}
					thinking, _ := req["thinking"].(map[string]any)
					wantEffort := "low"
					if isReportCalibrationFixtureRequest(req) {
						wantEffort = "high"
					}
					if req["model"] != "fixture" || thinking["type"] != "enabled" || req["reasoning_effort"] != wantEffort {
						t.Errorf("changed selected report reasoning policy: model=%v thinking=%v effort=%v", req["model"], thinking, req["reasoning_effort"])
					}
					if isReportCalibrationFixtureRequest(req) {
						reviews++
						if req["max_tokens"] != float64(tc.wantReview) {
							t.Errorf("critic wire budget=%v, want %d", req["max_tokens"], tc.wantReview)
						}
						messages, _ := req["messages"].([]any)
						var actual reportCalibrationInput
						if len(messages) != 2 || json.Unmarshal([]byte(fmt.Sprint(messages[1].(map[string]any)["content"])), &actual) != nil || actual.Draft != body || actual.Question != "合成预算测试" || string(actual.Inputs) != `{}` {
							t.Error("critic budget changed or omitted confirmed inputs/full draft")
						}
						if tc.finish == "length" {
							_ = json.NewEncoder(w).Encode(map[string]any{"model": "fixture", "choices": []any{map[string]any{"message": map[string]any{"content": tc.reviewContent}, "finish_reason": "length"}}, "usage": map[string]any{"prompt_tokens": 7256, "completion_tokens": 32767, "completion_tokens_details": map[string]any{"reasoning_tokens": 32767}}})
						} else {
							writeReportCalibrationFixture(w, `{"issues":[]}`)
						}
					} else {
						writers++
						if req["max_tokens"] != float64(tc.wantWriter) {
							t.Errorf("writer wire budget=%v, want %d", req["max_tokens"], tc.wantWriter)
						}
						event, _ := json.Marshal(map[string]any{"model": "fixture", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "content": string(rawBody)}, "finish_reason": "stop"}}, "usage": map[string]any{"prompt_tokens": 3, "completion_tokens": 2, "completion_tokens_details": map[string]any{"reasoning_tokens": 1}}})
						w.Header().Set("Content-Type", "text/event-stream")
						_, _ = fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", event)
					}
				}
				response := w.Result()
				response.Request = r
				return response, nil
			})})
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			for _, stage := range []string{"checking", "calculating", "writing"} {
				mock.ExpectExec("UPDATE ai_report SET generation_stage").WithArgs(stage, int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			s := &svc.ServiceContext{DB: sqlx.NewSqlConnFromDB(db), Provider: p, Guard: agent.NewGuard(20000, nil), SkillModel: reportSkills{&model.AISkill{Code: "fixture-topic", Status: model.SkillStatusEnabled, InputSchema: `{"fields":[]}`, ToolConfig: `{"enabled":false}`}}}
			s.AIConfig.ContextWindow, s.AIConfig.ComplexOutputTokens = tc.window, tc.writerOutput
			if tc.catalog {
				s.Models = provider.NewCatalog(p)
			}
			got, doc, usage, err := executeReport(context.Background(), s, &Report{ID: 1, SkillCode: "fixture-topic", Question: "合成预算测试", InputsJSON: `{}`, ChaptersJSON: `["依据","对照","行动"]`})
			var audit map[string]any
			for _, evidence := range doc.Evidence {
				if evidence.Tool == "report_calibration" {
					_ = json.Unmarshal([]byte(evidence.Text), &audit)
				}
			}
			if writers != 1 || reviews != 1 || doc.ModelCalls != 2 || doc.ToolCalls != 0 {
				t.Fatalf("changed execution bounds: err=%v writers=%d reviews=%d models=%d tools=%d", err, writers, reviews, doc.ModelCalls, doc.ToolCalls)
			}
			if tc.finish == "length" {
				if err == nil || got != (reportBody{}) || audit["status"] == "passed" || audit["reviews"] != float64(1) || audit["protocolRepairs"] != float64(0) || usage.PromptTokens != 7259 || usage.CompletionTokens != 32769 || usage.ReasoningTokens != 32768 {
					t.Fatalf("truncated critic accepted/retried/lost usage: err=%v bodyEmpty=%t audit=%v usage=%+v", err, got == (reportBody{}), audit, usage)
				}
			} else if err != nil || got != body || audit["status"] != "passed" || usage.PromptTokens != 14 || usage.CompletionTokens != 9 || usage.ReasoningTokens != 4 {
				t.Fatalf("valid report or accounting changed: err=%v bodyEqual=%t audit=%v usage=%+v", err, got == body, audit, usage)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
