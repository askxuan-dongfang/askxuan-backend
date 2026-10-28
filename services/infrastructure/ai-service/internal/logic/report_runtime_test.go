package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/reportdoc"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type reportSkills []*model.AISkill

func (s reportSkills) List(context.Context, string) ([]*model.AISkill, error) { return s, nil }
func (s reportSkills) FindByCode(_ context.Context, code string) (*model.AISkill, error) {
	for _, x := range s {
		if x.Code == code {
			return x, nil
		}
	}
	return nil, fmt.Errorf("missing %s", code)
}
func TestReportRunsHarnessWithComputedCharts(t *testing.T) {
	for _, code := range []string{"bazi", "marriage", "ziwei"} {
		t.Run(code, func(t *testing.T) { testReportHarness(t, code, 0, 0) })
	}
}
func TestMarriageReportRepairsOnceWithoutRecalculating(t *testing.T) {
	t.Run("corrected", func(t *testing.T) { testReportHarness(t, "marriage", 1, 0) })
	t.Run("still_conflicts", func(t *testing.T) { testReportHarness(t, "marriage", 2, 0) })
}
func TestReportCalibrationRepairsOnceWithoutRecalculating(t *testing.T) {
	t.Run("corrected", func(t *testing.T) { testReportHarness(t, "marriage", 0, 1) })
	t.Run("still_conflicts", func(t *testing.T) { testReportHarness(t, "marriage", 0, 2) })
}
func TestReportCalibrationRejectsInvalidReviewAfterBoundedProtocolRepair(t *testing.T) {
	for _, failure := range []string{"length", "provider_error", "empty_content", "empty_length", "empty_content_filter", "empty_tool_calls", "empty_function_call", "empty_unknown_finish", "empty_then_invalid", "invalid_then_empty", "ungrounded_quote", "null_issues"} {
		t.Run(failure, func(t *testing.T) { testReportHarness(t, "marriage", 0, 0, failure) })
	}
}
func TestReportCalibrationProtocolRepairDoesNotRewriteOrRecalculate(t *testing.T) {
	for _, response := range []string{"invalid_then_valid", "empty_then_valid", "empty_missing_finish_then_valid"} {
		t.Run(response, func(t *testing.T) { testReportHarness(t, "marriage", 0, 0, response) })
	}
}
func TestReportCalibrationAllowsOneProtocolRepairPerDraft(t *testing.T) {
	for _, response := range []string{"invalid_each_round_then_valid", "empty_each_round_then_valid"} {
		t.Run(response, func(t *testing.T) { testReportHarness(t, "marriage", 0, 1, response) })
	}
}

func isReportCalibrationFixtureRequest(req map[string]any) bool {
	raw, _ := json.Marshal(req["messages"])
	return req["stream"] == false || strings.Contains(string(raw), "report_calibration_v1")
}

func writeReportCalibrationFixture(w http.ResponseWriter, content string, finishReasons ...string) {
	finishReason := "stop"
	if len(finishReasons) > 0 {
		finishReason = finishReasons[0]
	}
	w.Header().Set("Content-Type", "application/json")
	choice := map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": content}}
	if finishReason != "" {
		choice["finish_reason"] = finishReason
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id": "calibration-fixture", "object": "chat.completion", "model": "fixture",
		"choices": []any{choice},
		"usage":   map[string]any{"prompt_tokens": 11, "completion_tokens": 7, "prompt_cache_hit_tokens": 5, "prompt_cache_miss_tokens": 6, "completion_tokens_details": map[string]any{"reasoning_tokens": 3}},
	})
}

func testReportHarness(t *testing.T, code string, conflictingResponses, calibrationConflicts int, calibrationFailures ...string) {
	calibrationFailure := ""
	if len(calibrationFailures) > 0 {
		calibrationFailure = calibrationFailures[0]
	}
	fixture, e := os.ReadFile("../reportdoc/testdata/synthetic.json")
	if e != nil {
		t.Fatal(e)
	}
	var results map[string]json.RawMessage
	_ = json.Unmarshal(fixture, &results)
	results["ziwei_horoscope"] = json.RawMessage(`{"运限叠宫":[{"层次":"流年","落入本命宫位":"命宫","干支":"甲子"}]}`)
	var targetDates []string
	var toolCalls, modelCalls, generationCalls, calibrationCalls atomic.Int32
	mcp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		toolCalls.Add(1)
		if req.Params.Name == "ziwei_horoscope" {
			targetDates = append(targetDates, fmt.Sprint(req.Params.Arguments["targetDate"]))
		}
		if req.Params.Arguments["birthYear"] != float64(1990) && (code != "marriage" || req.Params.Arguments["birthYear"] != float64(1992)) {
			t.Error("lost confirmed birth date")
		}
		var result map[string]any
		_ = json.Unmarshal(results[req.Params.Name], &result)
		result["fixtureBirthYear"] = req.Params.Arguments["birthYear"]
		result["fixtureTargetDate"] = req.Params.Arguments["targetDate"]
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"structuredContent": result}})
	}))
	defer mcp.Close()
	body := reportBody{Summary: strings.Repeat("根据已确认资料和工具结果整理阅读线索。", 3), Content: "## 计算依据\n" + strings.Repeat("依据计算数据解释传统文化术语，不作结果保证。", 20) + "\n## 阶段比较\n| 阶段 | 参考 |\n| --- | --- |\n| 当前 | 核对现实条件 |\n## 行动建议\n记录并复盘。"}
	rawBody, _ := json.Marshal(body)
	var firstCalibrationRequest map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		modelCalls.Add(1)
		if isReportCalibrationFixtureRequest(req) {
			n := calibrationCalls.Add(1)
			if n%2 == 1 {
				firstCalibrationRequest = req
			} else if calibrationFailure != "" {
				before, _ := firstCalibrationRequest["messages"].([]any)
				after, _ := req["messages"].([]any)
				if len(after) != len(before)+1 || !reflect.DeepEqual(after[:len(before)], before) {
					t.Error("protocol correction replaced the original calibration rules or draft")
				}
				for key, value := range firstCalibrationRequest {
					if key != "messages" && !reflect.DeepEqual(req[key], value) {
						t.Errorf("protocol correction changed request policy field %s", key)
					}
				}
			}
			raw, _ := json.Marshal(req["messages"])
			if !strings.Contains(string(raw), "report_calibration_v1") || !strings.Contains(string(raw), "fixtureBirthYear") || ((code == "bazi" || code == "marriage") && !strings.Contains(string(raw), "四柱")) {
				t.Error("calibration did not receive current calculation evidence")
			}
			response := `{"issues":[]}`
			conflictingDraft := calibrationConflicts > 0 && (generationCalls.Load() == 2 || calibrationConflicts > 1)
			if strings.Contains(string(raw), "对方伤官透于时") != conflictingDraft {
				t.Error("calibration did not receive the current draft")
			}
			if conflictingDraft {
				response = `{"issues":[{"quote":"对方伤官透于时","basis":"对方时柱为甲辰，时干甲为正印，辰中戊为伤官；不能称伤官透干。"}]}`
			}
			finishReason := "stop"
			switch calibrationFailure {
			case "length":
				finishReason = "length"
			case "provider_error":
				http.Error(w, "fixture provider unavailable", http.StatusBadGateway)
				return
			case "empty_content":
				response = ""
			case "empty_length", "empty_content_filter", "empty_tool_calls", "empty_function_call":
				response, finishReason = "", strings.TrimPrefix(calibrationFailure, "empty_")
			case "empty_unknown_finish":
				response, finishReason = "", "untrusted_finish\nPRIVATE_REASONING"
			case "empty_then_valid", "empty_missing_finish_then_valid":
				if n == 1 {
					response = ""
					if calibrationFailure == "empty_missing_finish_then_valid" {
						finishReason = "" // Reproduce absent metadata; do not invent stop.
					}
				}
			case "empty_then_invalid":
				response = `{"issues":null}`
				if n == 1 {
					response = ""
				}
			case "invalid_then_empty":
				response = ""
				if n == 1 {
					response = `{"issues":null}`
				}
			case "ungrounded_quote":
				response = `{"issues":[{"quote":"审读伪造且不在草稿中的断言","basis":"审读声称存在的句子没有出现在报告草稿中。"}]}`
			case "null_issues":
				response = `{"issues":null}`
			case "invalid_then_valid":
				if n == 1 {
					response = `{"issues":null}`
				}
			case "invalid_each_round_then_valid":
				if n%2 == 1 {
					response = `{"issues":null}`
				}
			case "empty_each_round_then_valid":
				if n%2 == 1 {
					response = ""
				}
			}
			if n == 2 && (calibrationFailure == "ungrounded_quote" || calibrationFailure == "null_issues" || calibrationFailure == "invalid_then_valid" || calibrationFailure == "invalid_then_empty") {
				if !strings.Contains(string(raw), "invalidReview") || !strings.Contains(string(raw), "validationError") || !strings.Contains(string(raw), "仅修正审读结果") {
					t.Error("protocol repair lost invalid response, parser feedback or review-only instruction")
				}
			}
			if n%2 == 0 && strings.HasPrefix(calibrationFailure, "empty_") && !strings.HasPrefix(calibrationFailure, "empty_content_") {
				if !strings.Contains(string(raw), "上次审读未返回可用JSON正文") || !strings.Contains(string(raw), "同一组审读规则") || !strings.Contains(string(raw), "只返回JSON对象") {
					t.Error("empty correction lost explicit JSON or unchanged calibration rules")
				}
			}
			writeReportCalibrationFixture(w, response, finishReason)
			return
		}
		n := generationCalls.Add(1)
		var delta map[string]any
		reason := "stop"
		if n%2 == 1 {
			reason = "tool_calls"
			delta = map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": "read_1", "type": "function", "function": map[string]any{"name": "read_report", "arguments": "{}"}}}}
		} else {
			b, _ := json.Marshal(req["messages"])
			if !strings.Contains(string(b), "current_calculation") || !strings.Contains(string(b), "四柱") || (code == "bazi" && !strings.Contains(string(b), "大运列表")) || (code == "marriage" && !strings.Contains(string(b), "对方 · 四柱命盘")) {
				t.Error("agent did not receive calculation evidence")
			}
			response := rawBody
			if conflictingResponses > 0 && (n == 2 || conflictingResponses > 1) {
				wrong := body
				wrong.Summary = "女方八字与紫微命盘均未取得，因此本报告不是双人合盘，仅男方的关系模式分析。"
				response, _ = json.Marshal(wrong)
			}
			if calibrationConflicts > 0 && (n == 2 || calibrationConflicts > 1) {
				wrong := body
				wrong.Summary = "对方伤官透于时。" + wrong.Summary
				response, _ = json.Marshal(wrong)
			}
			delta = map[string]any{"role": "assistant", "content": string(response)}
		}
		w.Header().Set("Content-Type", "text/event-stream")
		event, _ := json.Marshal(map[string]any{"id": "fixture", "object": "chat.completion.chunk", "model": "fixture", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": reason}}, "usage": map[string]any{"prompt_tokens": 3, "completion_tokens": 2, "completion_tokens_details": map[string]any{"reasoning_tokens": 1}}})
		fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", event)
	}))
	defer server.Close()
	p := provider.NewOpenAICompatible(server.URL, "fixture", "fixture", "")
	p.SetHTTPClient(server.Client())
	p.UseStandardParameters()
	db, m, _ := sqlmock.New()
	defer db.Close()
	for _, stage := range []string{"checking", "calculating", "writing"} {
		m.ExpectExec("UPDATE ai_report SET generation_stage").WithArgs(stage, int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	}
	skills := reportSkills{}
	for _, code := range []string{"bazi", "bazi_dayun", "ziwei", "ziwei_horoscope"} {
		skills = append(skills, &model.AISkill{Code: code, Name: code, Status: "enabled", InputSchema: reportToolSchema(code), ToolConfig: `{"enabled":true,"server":"taibu","tool":"` + code + `"}`})
	}
	skills = append(skills, &model.AISkill{Code: "marriage", Status: "enabled", InputSchema: `{"fields":[]}`, ToolConfig: `{"enabled":false}`})
	s := &svc.ServiceContext{DB: sqlx.NewSqlConnFromDB(db), SkillModel: skills, Guard: agent.NewGuard(20000, nil), Provider: p, MCP: agent.NewMCPClient(true, mcp.URL, 10)}
	s.AIConfig.ContextWindow = 1048576
	r := &Report{ID: 1, SkillCode: code, Question: "生成八字报告", InputsJSON: `{"calendarType":"solar","birthDate":"1990-01-02","birthTime":"08:30","gender":"male"}`, ChaptersJSON: `["计算依据","阶段比较","行动建议"]`}
	if code == "marriage" {
		r.InputsJSON = `{"calendarType":"solar","birthDate":"1990-01-02","birthTime":"08:30","gender":"male","partnerCalendarType":"lunar","partnerBirthDate":"1992-02-30","partnerBirthTime":"10:30","partnerGender":"female"}`
	}
	if code == "ziwei" {
		r.InputsJSON = strings.TrimSuffix(r.InputsJSON, "}") + `,"targetDate":"2026-10-26"}`
	}
	result, doc, usage, e := executeReport(context.Background(), s, r)
	chargedReviews := calibrationCalls.Load()
	if calibrationFailure == "provider_error" {
		chargedReviews-- // An HTTP failure returns no token usage, but still counts as a model call.
	}
	if usage.PromptTokens != int(generationCalls.Load())*3+int(chargedReviews)*11 || usage.CompletionTokens != int(generationCalls.Load())*2+int(chargedReviews)*7 || usage.PromptCacheHitTokens != int(chargedReviews)*5 || usage.PromptCacheMissTokens != int(generationCalls.Load())*3+int(chargedReviews)*6 || usage.ReasoningTokens != int(generationCalls.Load())+int(chargedReviews)*3 {
		t.Fatalf("generation/calibration usage lost: %+v generations=%d calibrations=%d", usage, generationCalls.Load(), calibrationCalls.Load())
	}
	if doc.ModelCalls != int(modelCalls.Load()) {
		t.Fatalf("model calls lost: document=%d actual=%d", doc.ModelCalls, modelCalls.Load())
	}
	protocolSuccess := calibrationFailure == "invalid_then_valid" || calibrationFailure == "invalid_each_round_then_valid" || calibrationFailure == "empty_then_valid" || calibrationFailure == "empty_missing_finish_then_valid" || calibrationFailure == "empty_each_round_then_valid"
	if calibrationFailure != "" && !protocolSuccess {
		wantReviews := int32(2)
		if calibrationFailure == "length" || calibrationFailure == "provider_error" || calibrationFailure == "empty_length" || calibrationFailure == "empty_content_filter" || calibrationFailure == "empty_tool_calls" || calibrationFailure == "empty_function_call" || calibrationFailure == "empty_unknown_finish" {
			wantReviews = 1
		}
		if e == nil || modelCalls.Load() != 2+wantReviews || generationCalls.Load() != 2 || calibrationCalls.Load() != wantReviews || toolCalls.Load() != 2 || result.Summary != "" || result.Content != "" {
			t.Fatalf("invalid review was published or exceeded its repair bound: %v models=%d generations=%d calibrations=%d tools=%d", e, modelCalls.Load(), generationCalls.Load(), calibrationCalls.Load(), toolCalls.Load())
		}
		var audit map[string]any
		for _, evidence := range doc.Evidence {
			if evidence.Tool == "report_calibration" {
				if err := json.Unmarshal([]byte(evidence.Text), &audit); err != nil {
					t.Fatal(err)
				}
			}
		}
		if audit["status"] == "passed" || audit["reviews"] != float64(wantReviews) || audit["protocolRepairs"] != float64(wantReviews-1) {
			t.Fatalf("failed review lost its shared correction budget or audit counters: %v", audit)
		}
		if e = m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
		return
	}
	if conflictingResponses == 2 {
		if e == nil || modelCalls.Load() != 4 || calibrationCalls.Load() != 0 || toolCalls.Load() != 2 {
			t.Fatalf("contradiction not bounded: %v models=%d tools=%d", e, modelCalls.Load(), toolCalls.Load())
		}
		return
	}
	if calibrationConflicts == 2 {
		if e == nil || modelCalls.Load() != 6 || calibrationCalls.Load() != 2 || toolCalls.Load() != 2 {
			t.Fatalf("semantic contradiction not bounded: %v models=%d calibrations=%d tools=%d", e, modelCalls.Load(), calibrationCalls.Load(), toolCalls.Load())
		}
		return
	}
	if e != nil {
		t.Fatal(e)
	}
	expectedTools, expectedModels := int32(2), int32(3)
	if code == "ziwei" {
		expectedTools = 3
		if strings.Join(targetDates, ",") != "2026-10-26,2027-10-26" {
			t.Fatalf("missing year comparison: %v", targetDates)
		}
	}
	if conflictingResponses > 0 {
		expectedModels = 5
	}
	if calibrationConflicts > 0 {
		expectedModels = 6
	}
	if protocolSuccess {
		expectedModels = 4
		wantReviews, wantGenerations, wantProtocolRepairs := int32(2), int32(2), 1
		if calibrationFailure == "invalid_each_round_then_valid" || calibrationFailure == "empty_each_round_then_valid" {
			expectedModels = 8
			wantReviews, wantGenerations, wantProtocolRepairs = 4, 4, 2
		}
		var audit map[string]any
		statuses := []string{}
		for _, evidence := range doc.Evidence {
			if evidence.Tool == "report_calibration" {
				if err := json.Unmarshal([]byte(evidence.Text), &audit); err != nil {
					t.Fatal(err)
				}
				statuses = append(statuses, fmt.Sprint(audit["status"]))
			}
		}
		if calibrationCalls.Load() != wantReviews || generationCalls.Load() != wantGenerations || audit["status"] != "passed" || audit["reviews"] != float64(wantReviews) || audit["protocolRepairs"] != float64(wantProtocolRepairs) || audit["repaired"] != (calibrationConflicts > 0) {
			t.Fatalf("protocol repair changed the draft or lost its audit counters: %v generations=%d reviews=%d", audit, generationCalls.Load(), calibrationCalls.Load())
		}
		wantStatuses := "invalid_review,passed"
		if calibrationFailure == "invalid_each_round_then_valid" || calibrationFailure == "empty_each_round_then_valid" {
			wantStatuses = "invalid_review,needs_repair,invalid_review,passed"
		}
		if strings.Join(statuses, ",") != wantStatuses {
			t.Fatalf("writer repair discarded prior private audit: %v", statuses)
		}
	}
	if result.Summary != body.Summary || result.Content != body.Content || doc.Runtime != "harness" || doc.ToolCalls != int(expectedTools) || toolCalls.Load() != expectedTools || modelCalls.Load() != expectedModels || len(doc.Blocks) < 4 {
		t.Fatalf("report pipeline incorrect tools=%d models=%d blocks=%d", toolCalls.Load(), modelCalls.Load(), len(doc.Blocks))
	}
	if code == "marriage" {
		self, partner := false, false
		for _, b := range doc.Blocks {
			self = self || strings.HasPrefix(b.Title, "你 ·")
			partner = partner || strings.HasPrefix(b.Title, "对方 ·")
		}
		if !self || !partner {
			t.Fatal("participant charts not labelled")
		}
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestReportRandomToolCannotRedraw(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		fmt.Fprint(w, `{"result":{"content":[{"type":"text","text":"same draw"}]}}`)
	}))
	defer srv.Close()
	m := &reportMCP{s: &svc.ServiceContext{MCP: agent.NewMCPClient(true, srv.URL, 10)}, cache: map[string]string{}, doc: reportdoc.New()}
	for _, arg := range []string{`{"date":"2026-09-29T12:00"}`, `{"date":"2026-09-29T12:01"}`} {
		if _, e := m.Call(context.Background(), `{"enabled":true,"server":"taibu","tool":"liuyao"}`, arg); e != nil {
			t.Fatal(e)
		}
	}
	if calls.Load() != 1 {
		t.Fatal("report redrew a random result")
	}
}

func reportToolSchema(code string) string {
	for _, d := range agent.ReviewedTools() {
		if d.Code == code {
			raw, _ := json.Marshal(d.InputSchema)
			return string(raw)
		}
	}
	return `{"fields":[]}`
}

// Exercise the real report -> builtin -> evidence -> Eino model path. The model
// transport is synthetic; the builtin algorithms and chart normalizer are real.
func TestTopicReportsRequireAndReadToolEvidence(t *testing.T) {
	for _, tc := range []struct{ code, input string }{
		{"naming", `{"mode":"review","surname":"李","name":"李明，李铭"}`},
		{"fengshui", `{"scene":"home","daylight":"dim"}`},
		{"dream", `{"dream":"梦见赶车","feeling":"着急"}`},
		{"date_select", `{"event":"嫁娶","targetDate":"2026-09-29"}`},
		{"fortune", `{"targetDate":"2026-09-29"}`},
	} {
		t.Run(tc.code, func(t *testing.T) {
			var calls, generationCalls atomic.Int32
			body := reportBody{Summary: "依据用户已确认资料和工具返回结果整理，区分实际计算依据与待补充的信息。", Content: "## 核验依据\n" + strings.Repeat("这里只解释工具实际返回的信息，缺失资料保留待确认。", 12) + "\n## 结果对照\n" + strings.Repeat("依据来源与用户原述分开呈现，不添加未经验证的计算结果。", 12) + "\n## 下一步\n补充尚未确认的信息后再继续分析。"}
			encoded, _ := json.Marshal(body)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req map[string]any
				_ = json.NewDecoder(r.Body).Decode(&req)
				calls.Add(1)
				if isReportCalibrationFixtureRequest(req) {
					writeReportCalibrationFixture(w, `{"issues":[]}`)
					return
				}
				var delta map[string]any
				reason := "stop"
				if generationCalls.Add(1) == 1 {
					reason = "tool_calls"
					delta = map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": "read", "type": "function", "function": map[string]any{"name": "read_report", "arguments": "{}"}}}}
				} else {
					raw, _ := json.Marshal(req["messages"])
					if !strings.Contains(string(raw), "20260929.1") {
						t.Error("model never read versioned tool evidence")
					}
					delta = map[string]any{"role": "assistant", "content": string(encoded)}
				}
				w.Header().Set("Content-Type", "text/event-stream")
				event, _ := json.Marshal(map[string]any{"id": "fixture", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": reason}}})
				fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", event)
			}))
			defer server.Close()
			mcp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprint(w, `{"result":{"structuredContent":{"基础与个性化坐标":{"日期":"2026-09-29","日干支":"丙午"},"择日宜忌":{"宜":["嫁娶"],"忌":[]}}}}`)
			}))
			defer mcp.Close()
			p := provider.NewOpenAICompatible(server.URL, "fixture", "fixture", "")
			p.SetHTTPClient(server.Client())
			p.UseStandardParameters()
			db, m, _ := sqlmock.New()
			defer db.Close()
			for _, stage := range []string{"checking", "calculating", "writing"} {
				m.ExpectExec("UPDATE ai_report SET generation_stage").WithArgs(stage, int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			skill := &model.AISkill{Code: tc.code, Name: tc.code, Status: "enabled", InputSchema: reportToolSchema(tc.code), ToolConfig: `{"enabled":true,"server":"builtin","tool":"` + tc.code + `"}`}
			s := &svc.ServiceContext{DB: sqlx.NewSqlConnFromDB(db), SkillModel: reportSkills{skill}, Guard: agent.NewGuard(20000, nil), Provider: p, MCP: agent.NewMCPClient(true, mcp.URL, 1)}
			s.AIConfig.ContextWindow = 1048576
			result, doc, usage, e := executeReport(context.Background(), s, &Report{ID: 1, SkillCode: tc.code, Question: "整理专题", InputsJSON: tc.input, ChaptersJSON: `["核验依据"]`})
			if e != nil || result.Content != body.Content || len(doc.Blocks) == 0 || doc.Blocks[0].Kind != "table" || doc.Blocks[0].Source != tc.code || calls.Load() != 3 || doc.ModelCalls != 3 || usage.PromptTokens != 11 || usage.CompletionTokens != 7 || usage.PromptCacheHitTokens != 5 || usage.PromptCacheMissTokens != 6 || usage.ReasoningTokens != 3 {
				t.Fatalf("topic %s evidence pipeline: %+v %v", tc.code, doc, e)
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestReportOmitsCalculationsRequiringUnconfirmedFacts(t *testing.T) {
	skills := reportSkills{}
	for _, code := range []string{"ziwei", "ziwei_horoscope", "bazi", "bazi_dayun", "qimen", "liuyao", "naming"} {
		skills = append(skills, &model.AISkill{Code: code, Status: "enabled", InputSchema: reportToolSchema(code)})
	}
	facts := map[string]any{"calendarType": "solar", "birthDate": "1990-01-02", "birthTime": "08:30", "gender": "male"}
	guard := agent.NewGuard(20000, nil)
	codes := func() map[string]bool {
		out := map[string]bool{}
		for _, s := range reportAvailableSkills(skills, facts, "生成专题报告", guard) {
			out[s.Code] = true
		}
		return out
	}
	got := codes()
	for _, code := range []string{"ziwei", "bazi", "bazi_dayun"} {
		if !got[code] {
			t.Errorf("lost available calculation %s", code)
		}
	}
	for _, code := range []string{"ziwei_horoscope", "qimen", "liuyao", "naming"} {
		if got[code] {
			t.Errorf("exposed calculation without confirmed inputs: %s", code)
		}
	}
	facts["targetDate"] = "2026-10-20"
	if !codes()["ziwei_horoscope"] {
		t.Fatal("confirmed target date should enable transits")
	}
	facts["targetDate"] = "invalid"
	if codes()["ziwei_horoscope"] {
		t.Fatal("invalid target date exposed transits")
	}
}
