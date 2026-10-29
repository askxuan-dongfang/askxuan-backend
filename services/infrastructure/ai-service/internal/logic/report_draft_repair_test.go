package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestReportDraftRepairSharesSingleWriterRewrite(t *testing.T) {
	for _, tc := range []struct {
		name, first, second string
		planning            int
		semantic            bool
		wantSuccess         bool
		wantReviews         int
	}{
		{name: "four_calls_then_format_repair", first: "invalid", planning: 3, wantSuccess: true, wantReviews: 1},
		{name: "incomplete_then_valid", first: "incomplete", wantSuccess: true, wantReviews: 1},
		{name: "two_invalid_drafts", first: "invalid", second: "invalid"},
		{name: "citation_repair_then_invalid", first: "citation", second: "invalid"},
		{name: "format_repair_then_citation_conflict", first: "invalid", second: "citation"},
		{name: "semantic_repair_then_invalid", second: "invalid", semantic: true, wantReviews: 1},
		{name: "format_repair_then_semantic_conflict", first: "invalid", semantic: true, wantReviews: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture, err := os.ReadFile("../reportdoc/testdata/synthetic.json")
			if err != nil {
				t.Fatal(err)
			}
			var calculations map[string]json.RawMessage
			if err := json.Unmarshal(fixture, &calculations); err != nil {
				t.Fatal(err)
			}
			mcpCalls := 0
			mcp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req struct {
					Params struct{ Name string } `json:"params"`
				}
				_ = json.NewDecoder(r.Body).Decode(&req)
				mcpCalls++
				_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"structuredContent": calculations[req.Params.Name]}})
			}))
			t.Cleanup(mcp.Close)
			body := reportBody{Summary: strings.Repeat("合成摘要只为校验修复次数。", 3), Content: "## 计算依据\n" + strings.Repeat("保留本次计算与原文，不代表真实模型质量。", 25) + "\n## 选择比较\n按确认条件比较。\n## 行动建议\n逐项记录。"}
			rawBody, _ := json.Marshal(body)
			writers, reviews := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req map[string]any
				_ = json.NewDecoder(r.Body).Decode(&req)
				wantEffort := "low"
				if isReportCalibrationFixtureRequest(req) {
					wantEffort = "high"
				}
				thinking, _ := req["thinking"].(map[string]any)
				if req["max_tokens"] != float64(16384) || req["reasoning_effort"] != wantEffort || thinking["type"] != "enabled" {
					t.Error("format correction changed output or reasoning policy")
				}
				if isReportCalibrationFixtureRequest(req) {
					reviews++
					result := `{"issues":[]}`
					if tc.semantic {
						raw, _ := json.Marshal(map[string]any{"issues": []reportCalibrationIssue{{Quote: "合成摘要", Basis: "合成语义冲突，不代表真实报告。"}}})
						result = string(raw)
					}
					writeReportCalibrationFixture(w, result)
					return
				}
				writers++
				delta, finish := map[string]any{"role": "assistant", "content": string(rawBody)}, "stop"
				if writers <= tc.planning {
					delta = map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": fmt.Sprint("read-", writers), "type": "function", "function": map[string]any{"name": "read_report", "arguments": "{}"}}}}
					finish = "tool_calls"
				} else {
					kind := tc.second
					if writers == tc.planning+1 {
						kind = tc.first
					} else {
						messages, _ := json.Marshal(req["messages"])
						if !strings.Contains(string(messages), "1990-01-02") || !strings.Contains(string(messages), "不要重新计算") {
							t.Error("rewrite lost confirmed inputs or no-recalculation instruction")
						}
						if tc.first != "" && !strings.Contains(string(messages), "summary") {
							t.Error("format rewrite lost its JSON contract")
						}
					}
					switch kind {
					case "invalid":
						delta["content"] = "PRIVATE DRAFT AND PARSE INPUT: invalid JSON"
					case "incomplete":
						delta["content"] = `{"summary":"短","content":"PRIVATE SHORT BODY"}`
					case "citation":
						wrong := body
						wrong.Content += "[资料999]"
						raw, _ := json.Marshal(wrong)
						delta["content"] = string(raw)
					}
				}
				w.Header().Set("Content-Type", "text/event-stream")
				event, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}, "usage": map[string]any{"prompt_tokens": 3, "completion_tokens": 2, "completion_tokens_details": map[string]any{"reasoning_tokens": 1}}})
				_, _ = fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", event)
			}))
			t.Cleanup(server.Close)
			p := provider.NewOpenAICompatible(server.URL, "fixture", "fixture", "")
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			for _, stage := range []string{"checking", "calculating", "writing"} {
				mock.ExpectExec("UPDATE ai_report SET generation_stage").WithArgs(stage, int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			skills := reportSkills{}
			for _, code := range []string{"bazi", "bazi_dayun"} {
				skills = append(skills, &model.AISkill{Code: code, Status: model.SkillStatusEnabled, InputSchema: reportToolSchema(code), ToolConfig: `{"enabled":true,"server":"taibu","tool":"` + code + `"}`})
			}
			s := &svc.ServiceContext{DB: sqlx.NewSqlConnFromDB(db), SkillModel: skills, Provider: p, MCP: agent.NewMCPClient(true, mcp.URL, 10), Guard: agent.NewGuard(20000, nil)}
			s.AIConfig.ContextWindow = 1048576
			got, doc, usage, err := executeReport(context.Background(), s, &Report{ID: 1, SkillCode: "bazi", Question: "PRIVATE USER QUESTION", InputsJSON: `{"calendarType":"solar","birthDate":"1990-01-02","birthTime":"08:30","gender":"male"}`, ChaptersJSON: `["计算依据","选择比较","行动建议"]`})
			if writers != tc.planning+2 || reviews != tc.wantReviews || doc.ModelCalls != writers+reviews || mcpCalls != 2 || doc.ToolCalls != 2 {
				t.Fatalf("shared rewrite/cached calculation bound changed: err=%v writers=%d reviews=%d models=%d mcp=%d tools=%d", err, writers, reviews, doc.ModelCalls, mcpCalls, doc.ToolCalls)
			}
			if usage.PromptTokens != writers*3+reviews*11 || usage.CompletionTokens != writers*2+reviews*7 || usage.ReasoningTokens != writers+reviews*3 {
				t.Fatal("format repair lost charged model usage")
			}
			if tc.wantSuccess {
				if err != nil || got != body {
					t.Fatalf("single format rewrite did not finish: err=%v bodyEqual=%t", err, got == body)
				}
			} else if err == nil || got != (reportBody{}) {
				t.Fatal("invalid second draft or semantic conflict was published")
			}
			audits := []map[string]any{}
			for _, evidence := range doc.Evidence {
				if evidence.Tool == "report_calibration" {
					var audit map[string]any
					_ = json.Unmarshal([]byte(evidence.Text), &audit)
					if audit["phase"] != nil {
						t.Fatal("already calibrated draft gained an extra early failure diagnostic")
					}
					audits = append(audits, audit)
				}
			}
			if len(audits) < 2 || audits[0]["status"] != "needs_repair" || audits[len(audits)-1]["repaired"] != true {
				t.Fatal("rewrite discarded the first diagnostic or shared repair marker")
			}
			if !tc.wantSuccess {
				raw, _ := json.Marshal(reportFailureAudit(doc))
				for _, private := range []string{"PRIVATE DRAFT", "PRIVATE USER", "PRIVATE SHORT BODY"} {
					if strings.Contains(string(raw), private) {
						t.Fatal("failed format audit leaked private report text")
					}
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
