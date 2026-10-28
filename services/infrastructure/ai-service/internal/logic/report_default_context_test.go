package logic

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type defaultContextTransport func(*http.Request) (*http.Response, error)

func (f defaultContextTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Use the same migrated dream fields and provider exchange as the MySQL gate;
// no listener, database server, model service or enlarged context is needed.
func TestReportDreamUsesDefaultContext(t *testing.T) {
	seed, err := os.ReadFile("../../../../../scripts/db/20260929_ai_topic_evidence.sql")
	if err != nil {
		t.Fatal(err)
	}
	var dreamSeed string
	for _, line := range strings.Split(string(seed), "\n") {
		if strings.HasPrefix(line, "UPDATE askxuan_ai.ai_skill SET ") && strings.HasSuffix(line, "WHERE code='dream';") {
			dreamSeed = line
		}
	}
	field := func(name string) string {
		t.Helper()
		match := regexp.MustCompile(name + `=CONVERT\(0x([0-9a-f]+) USING utf8mb4\)`).FindStringSubmatch(dreamSeed)
		if len(match) != 2 {
			t.Fatalf("missing migrated dream %s", name)
		}
		raw, err := hex.DecodeString(match[1])
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	skill := &model.AISkill{Code: "dream", Name: "周公解梦", Status: model.SkillStatusEnabled, InputSchema: field("input_schema"), PromptTemplate: field("prompt_template"), ToolConfig: field("tool_config")}
	productSeed, err := os.ReadFile("../../../../../scripts/db/20260929_ai_report_document.sql")
	if err != nil {
		t.Fatal(err)
	}
	chapters := regexp.MustCompile(`(?m)^INSERT IGNORE INTO askxuan_ai\.ai_report_product[^\n]*VALUES\('dream','[^']*','[^']*','(\[[^']*\])'`).FindSubmatch(productSeed)
	if len(chapters) != 2 {
		t.Fatal("missing migrated dream chapters")
	}
	body := reportBody{Summary: strings.Repeat("依据梦境描述整理线索。", 6), Content: "## 梦境线索\n" + strings.Repeat("这是隔离数据库验收内容，不代表真实模型质量。", 20) + "\n## 意象整理\n| 意象 | 解释 |\n| --- | --- |\n| 道路 | 个人联想 |\n## 行动建议\n记录与复盘。"}
	rawBody, _ := json.Marshal(body)
	writerCalls, reviews := 0, 0
	p := provider.NewOpenAICompatible("http://default-context.invalid", "fixture", "fixture", "")
	p.UseStandardParameters()
	p.SetHTTPClient(&http.Client{Transport: defaultContextTransport(func(r *http.Request) (*http.Response, error) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return nil, err
		}
		if req["max_tokens"] != float64(16384) {
			t.Errorf("changed default output budget: %v", req["max_tokens"])
		}
		response := httptest.NewRecorder()
		if isReportCalibrationFixtureRequest(req) {
			reviews++
			writeReportCalibrationFixture(response, `{"issues":[]}`)
		} else {
			writerCalls++
			delta := map[string]any{"role": "assistant", "content": string(rawBody)}
			finish := "stop"
			if writerCalls == 1 {
				finish = "tool_calls"
				delta = map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": "read_1", "type": "function", "function": map[string]any{"name": "read_report", "arguments": "{}"}}}}
			}
			event, _ := json.Marshal(map[string]any{"id": "fixture", "object": "chat.completion.chunk", "model": "fixture", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}})
			response.Header().Set("Content-Type", "text/event-stream")
			_, _ = fmt.Fprintf(response, "data: %s\n\ndata: [DONE]\n\n", event)
		}
		result := response.Result()
		result.Request = r
		return result, nil
	})})
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, stage := range []string{"checking", "calculating", "writing"} {
		mock.ExpectExec("UPDATE ai_report SET generation_stage").WithArgs(stage, int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	}
	s := &svc.ServiceContext{DB: sqlx.NewSqlConnFromDB(db), SkillModel: reportSkills{skill}, Guard: agent.NewGuard(2000, nil), Provider: p}
	inputs, err := s.Guard.Validate(agent.GuidedInputSchema(skill.Code, skill.InputSchema), "整理梦境中的道路", map[string]any{"dream": "在一条长路上散步"})
	if err != nil {
		t.Fatal(err)
	}
	r := &Report{ID: 1, UserID: "9501", SkillCode: "dream", Title: "周公解梦", Version: "3.0", Question: "整理梦境中的道路", InputsJSON: inputs, ChaptersJSON: string(chapters[1])}
	got, doc, usage, err := executeReport(context.Background(), s, r)
	var audit map[string]any
	for _, evidence := range doc.Evidence {
		if evidence.Tool == "report_calibration" {
			if parseErr := json.Unmarshal([]byte(evidence.Text), &audit); parseErr != nil {
				t.Fatal(parseErr)
			}
		}
	}
	if expectationErr := mock.ExpectationsWereMet(); expectationErr != nil {
		t.Fatal(expectationErr)
	}
	t.Logf("default context config=%d output config=%d writerHTTP=%d reviews=%d modelCalls=%d toolCalls=%d audit=%v", s.AIConfig.ContextWindow, s.AIConfig.ComplexOutputTokens, writerCalls, reviews, doc.ModelCalls, doc.ToolCalls, audit)
	if err != nil {
		t.Fatalf("default-context executeReport error=%v; empty body=%t", err, got == (reportBody{}))
	}
	if got != body || writerCalls != 2 || reviews != 1 || doc.ModelCalls != 3 || doc.ToolCalls != 1 || usage.PromptTokens != 11 || usage.CompletionTokens != 7 || audit["status"] != "passed" {
		t.Fatalf("incomplete report or changed execution/usage: bodyEqual=%t writerHTTP=%d reviews=%d doc=%+v usage=%+v audit=%v", got == body, writerCalls, reviews, doc, usage, audit)
	}
}
