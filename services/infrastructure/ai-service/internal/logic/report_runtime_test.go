package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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
	for _, code := range []string{"bazi", "marriage"} {
		t.Run(code, func(t *testing.T) { testReportHarness(t, code) })
	}
}
func testReportHarness(t *testing.T, code string) {
	fixture, e := os.ReadFile("../reportdoc/testdata/synthetic.json")
	if e != nil {
		t.Fatal(e)
	}
	var results map[string]json.RawMessage
	_ = json.Unmarshal(fixture, &results)
	var toolCalls, modelCalls atomic.Int32
	mcp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		toolCalls.Add(1)
		if req.Params.Arguments["birthYear"] != float64(1990) && (code != "marriage" || req.Params.Arguments["birthYear"] != float64(1992)) {
			t.Error("lost confirmed birth date")
		}
		var result map[string]any
		_ = json.Unmarshal(results[req.Params.Name], &result)
		result["fixtureBirthYear"] = req.Params.Arguments["birthYear"]
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"structuredContent": result}})
	}))
	defer mcp.Close()
	body := reportBody{Summary: strings.Repeat("根据已确认资料和工具结果整理阅读线索。", 3), Content: "## 计算依据\n" + strings.Repeat("依据计算数据解释传统文化术语，不作结果保证。", 20) + "\n## 阶段比较\n| 阶段 | 参考 |\n| --- | --- |\n| 当前 | 核对现实条件 |\n## 行动建议\n记录并复盘。"}
	rawBody, _ := json.Marshal(body)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		n := modelCalls.Add(1)
		var delta map[string]any
		reason := "stop"
		if n == 1 {
			reason = "tool_calls"
			delta = map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": "read_1", "type": "function", "function": map[string]any{"name": "read_report", "arguments": "{}"}}}}
		} else {
			b, _ := json.Marshal(req["messages"])
			if !strings.Contains(string(b), "四柱") || (code == "bazi" && !strings.Contains(string(b), "大运列表")) || (code == "marriage" && !strings.Contains(string(b), "对方的计算结果")) {
				t.Error("agent did not receive calculation evidence")
			}
			delta = map[string]any{"role": "assistant", "content": string(rawBody)}
		}
		w.Header().Set("Content-Type", "text/event-stream")
		event, _ := json.Marshal(map[string]any{"id": "fixture", "object": "chat.completion.chunk", "model": "fixture", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": reason}}})
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
	for _, code := range []string{"bazi", "bazi_dayun"} {
		skills = append(skills, &model.AISkill{Code: code, Name: code, Status: "enabled", InputSchema: reportToolSchema(code), ToolConfig: `{"enabled":true,"server":"taibu","tool":"` + code + `"}`})
	}
	skills = append(skills, &model.AISkill{Code: "marriage", Status: "enabled", InputSchema: `{"fields":[]}`, ToolConfig: `{"enabled":false}`})
	s := &svc.ServiceContext{DB: sqlx.NewSqlConnFromDB(db), SkillModel: skills, Guard: agent.NewGuard(20000, nil), Provider: p, MCP: agent.NewMCPClient(true, mcp.URL, 10)}
	s.AIConfig.ContextWindow = 1048576
	r := &Report{ID: 1, SkillCode: code, Question: "生成八字报告", InputsJSON: `{"calendarType":"solar","birthDate":"1990-01-02","birthTime":"08:30","gender":"male"}`, ChaptersJSON: `["计算依据","阶段比较","行动建议"]`}
	if code == "marriage" {
		r.InputsJSON = `{"calendarType":"solar","birthDate":"1990-01-02","birthTime":"08:30","gender":"male","partnerCalendarType":"lunar","partnerBirthDate":"1992-02-30","partnerBirthTime":"10:30","partnerGender":"female"}`
	}
	result, doc, _, e := executeReport(context.Background(), s, r)
	if e != nil {
		t.Fatal(e)
	}
	if result.Content != body.Content || doc.Runtime != "harness" || doc.ToolCalls != 2 || toolCalls.Load() != 2 || modelCalls.Load() != 2 || len(doc.Blocks) < 4 {
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
