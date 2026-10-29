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

func requireReportCitationWritingRules(t *testing.T, instruction string) {
	t.Helper()
	for _, rule := range []string{"仅标在片段实际支持的释义或忠实引文后", "来源列表和依据说明也用[资料N]", "不用裸资料1、资料2"} {
		if !strings.Contains(instruction, rule) {
			t.Errorf("missing citation scope/list rule %q", rule)
		}
	}
}

func TestReportCitationWritingRulesReachEveryTopic(t *testing.T) {
	for _, topic := range []string{"bazi", "marriage", "fengshui", "ziwei", "qimen", "tarot", "liuyao", "naming", "dream", "date_select", "fortune", "face_palm"} {
		t.Run(topic, func(t *testing.T) {
			requireReportCitationWritingRules(t, reportInstruction(&Report{SkillCode: topic}, &model.AISkill{}))
		})
	}
	doc := reportdoc.New()
	doc.Blocks = []reportdoc.Block{{Kind: "citations", Items: []reportdoc.Item{{Label: "资料1", Value: "平台方法说明"}, {Label: "资料2", Value: "忠實原文"}}}}
	for _, content := range []string{"本次依据：资料1为平台方法说明，资料2为原文。", "本次依据：[资料99]。", "“忠实原文”[资料2]"} {
		if validateReportCitations(reportBody{Content: content}, doc) == nil {
			t.Errorf("citation writing rule must not weaken validation: %q", content)
		}
	}
	if err := validateReportCitations(reportBody{Content: "本次依据：[资料1]为平台方法说明，[资料2]保留原文。"}, doc); err != nil {
		t.Fatal(err)
	}
}

// Exercise the same malformed-first-draft path as report99 without a live model.
func TestReportCitationWritingRulesReachFormatRepairAtDefaultContext(t *testing.T) {
	body := reportBody{Summary: "先核对课程安排，再比较试听与直接报名；未确认的现实条件仍需本人核实。", Content: "## 依据\n" + strings.Repeat("这份本地合成报告只验证资料编号指令传递，不代表真实模型质量。", 15) + "\n## 比较\n|选项|行动|\n|---|---|\n|试听|先核对时间|\n## 行动\n根据已确认资料安排。"}
	writers, reviews := 0, 0
	p := provider.NewOpenAICompatible("http://citation-writing.invalid", "fixture", "fixture", "")
	p.SetHTTPClient(&http.Client{Transport: defaultContextTransport(func(r *http.Request) (*http.Response, error) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return nil, err
		}
		if req["max_tokens"] != float64(16384) {
			t.Errorf("changed default output budget: %v", req["max_tokens"])
		}
		w := httptest.NewRecorder()
		if isReportCalibrationFixtureRequest(req) {
			reviews++
			writeReportCalibrationFixture(w, `{"issues":[]}`)
		} else {
			writers++
			messages := req["messages"].([]any)
			requireReportCitationWritingRules(t, fmt.Sprint(messages[0].(map[string]any)["content"]))
			text := "not JSON"
			if writers == 2 {
				requireReportCitationWritingRules(t, fmt.Sprint(messages[len(messages)-1].(map[string]any)["content"]))
				raw, _ := json.Marshal(body)
				text = string(raw)
			}
			event, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "content": text}, "finish_reason": "stop"}}})
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
	got, doc, _, err := executeReport(context.Background(), s, &Report{ID: 1, SkillCode: "fixture-topic", Question: "合成资料编号传递测试", InputsJSON: `{}`, ChaptersJSON: `["依据","比较","行动"]`})
	if err != nil || got != body || writers != 2 || reviews != 1 || doc.ModelCalls != 3 || doc.ToolCalls != 0 || s.AIConfig.ContextWindow != 0 {
		t.Fatalf("format repair changed draft, default32K or bounded calls: err=%v writers=%d reviews=%d models=%d tools=%d", err, writers, reviews, doc.ModelCalls, doc.ToolCalls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
