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
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Synthetic protocol failures verify correction transport and the existing call
// budget; report97's persisted generic error cannot identify its exact bad field.
func TestReportCalibrationProtocolFeedbackTargetsTheInvalidIssue(t *testing.T) {
	body := reportBody{Summary: "这是一份本地合成协议测试摘要，只核对格式反馈，不代表真实内容通过。", Content: "## 原文\n" + strings.Repeat("原", 220) + "\n**甲乙**  空白　标点。\n## 比较\n| 条件 | 操作 |\n| --- | --- |\n| 待确认 | 核对 |\n## 行动\n" + strings.Repeat("先核对实际输入，再列行动检查项。", 20)}
	tests := []struct {
		name   string
		issue  reportCalibrationIssue
		reason string
		hints  []string
		repeat bool
	}{
		{"quote_empty", reportCalibrationIssue{Quote: " ", Basis: "本地合成依据"}, "quote_empty", []string{"quote为空或仅空白", "移除", "逐字复制"}, false},
		{"basis_empty", reportCalibrationIssue{Quote: "**甲乙**", Basis: "　"}, "basis_empty", []string{"basis为空或仅空白", "真实依据", "500字"}, false},
		{"quote_too_long", reportCalibrationIssue{Quote: strings.Repeat("原", 201), Basis: "本地合成依据"}, "quote_too_long", []string{"201字", "200字", "连续原字串"}, false},
		{"basis_too_long", reportCalibrationIssue{Quote: "**甲乙**", Basis: strings.Repeat("据", 501)}, "basis_too_long", []string{"501字", "500字", "不新增依据"}, false},
		{"quote_not_literal", reportCalibrationIssue{Quote: "甲乙空白标点", Basis: "本地合成依据"}, "quote_not_literal", []string{"不是", "连续原字串", "保留Markdown"}, false},
		{"invalid_twice_still_fails", reportCalibrationIssue{Quote: "甲乙空白标点", Basis: "本地合成依据"}, "quote_not_literal", []string{"连续原字串"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			writers, reviews := 0, 0
			invalid, _ := json.Marshal(map[string]any{"issues": []reportCalibrationIssue{tc.issue}})
			p := provider.NewOpenAICompatible("http://protocol-feedback.invalid", "fixture", "fixture", "")
			p.SetHTTPClient(&http.Client{Transport: defaultContextTransport(func(r *http.Request) (*http.Response, error) {
				var req map[string]any
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					return nil, err
				}
				messages := req["messages"].([]any)
				critic := isReportCalibrationFixtureRequest(req)
				effort := "low"
				if critic {
					effort = "high"
				}
				if req["max_tokens"] != float64(16384) || req["reasoning_effort"] != effort {
					t.Error("protocol feedback changed output or reasoning policy")
				}
				w := httptest.NewRecorder()
				if critic {
					reviews++
					var input reportCalibrationInput
					if err := json.Unmarshal([]byte(fmt.Sprint(messages[1].(map[string]any)["content"])), &input); err != nil {
						return nil, err
					}
					if input.Draft != body || string(input.Inputs) != "{}" {
						t.Error("protocol repair changed the draft or confirmed inputs")
					}
					if reviews == 2 {
						correction := fmt.Sprint(messages[len(messages)-1].(map[string]any)["content"])
						for _, hint := range append([]string{"issue=1", "reason=" + tc.reason}, tc.hints...) {
							if !strings.Contains(correction, hint) {
								t.Errorf("missing targeted correction %q", hint)
							}
						}
					}
					response := string(invalid)
					if reviews == 2 && !tc.repeat {
						response = `{"issues":[]}`
					}
					writeReportCalibrationFixture(w, response)
				} else {
					writers++
					raw, _ := json.Marshal(body)
					event, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "content": string(raw)}, "finish_reason": "stop"}}, "usage": map[string]any{"prompt_tokens": 3, "completion_tokens": 2, "completion_tokens_details": map[string]any{"reasoning_tokens": 1}}})
					w.Header().Set("Content-Type", "text/event-stream")
					_, _ = fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", event)
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
			got, doc, usage, err := executeReport(context.Background(), s, &Report{ID: 1, SkillCode: "fixture-topic", Question: "本地合成协议反馈测试", InputsJSON: `{}`, ChaptersJSON: `["原文","比较","行动"]`})
			if writers != 1 || reviews != 2 || doc.ModelCalls != 3 || doc.ToolCalls != 0 || usage.PromptTokens != 25 || usage.CompletionTokens != 16 || usage.ReasoningTokens != 7 {
				t.Fatalf("protocol repair changed calls/tools/usage: writer=%d reviews=%d models=%d tools=%d usage=%+v err=%v", writers, reviews, doc.ModelCalls, doc.ToolCalls, usage, err)
			}
			if tc.repeat {
				if err == nil || got.Summary != "" || got.Content != "" {
					t.Fatal("second invalid review was delivered")
				}
			} else if err != nil || got != body {
				t.Fatalf("targeted correction did not preserve report: %v", err)
			}
			statuses := []string{}
			var last map[string]any
			for _, evidence := range doc.Evidence {
				if evidence.Tool == "report_calibration" {
					if e := json.Unmarshal([]byte(evidence.Text), &last); e != nil {
						t.Fatal(e)
					}
					statuses = append(statuses, fmt.Sprint(last["status"]))
				}
			}
			expected := "invalid_review,passed"
			if tc.repeat {
				expected = "invalid_review,invalid_review"
			}
			if strings.Join(statuses, ",") != expected || last["reviews"] != float64(2) || last["protocolRepairs"] != float64(1) || last["repaired"] != false {
				t.Fatalf("protocol audit/correction budget changed: %+v statuses=%v", last, statuses)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
