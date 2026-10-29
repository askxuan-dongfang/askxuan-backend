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

func TestReportGroundingAndParaphraseRulesReachEveryTopic(t *testing.T) {
	for _, topic := range []string{"bazi", "marriage", "fengshui", "ziwei", "qimen", "tarot", "liuyao", "naming", "dream", "date_select", "fortune", "face_palm"} {
		instruction := reportInstruction(&Report{SkillCode: topic}, &model.AISkill{})
		for _, required := range []string{"排盘与牌卦事实据本次计算", "现实情况只据用户已确认输入", "提示、显示或象征，不等于条件句", "不能由一项已知顾虑排除其它顾虑", "优先作明确标为释义或解释的转述", "必要的短引文须忠实保留原字形和本轮实际来源", "完整原文仍由平台知识依据图表忠实展示", "格式化执行状态标签"} {
			if !strings.Contains(instruction, required) {
				t.Errorf("%s writer missing grounding/source policy %q", topic, required)
			}
		}
		for _, obsolete := range []string{"现实事实只据本次输入和计算", "必要的直接引文", "转简体或改写须标为转写", "正文不重复直接引文", "正文只作明确标为释义或解释的转述", "不复引原句"} {
			if strings.Contains(instruction, obsolete) {
				t.Errorf("%s writer kept ambiguous or duplicate quote policy %q", topic, obsolete)
			}
		}
		dateRule := "只讨论本次实际日期和对应宜忌项"
		if strings.Contains(instruction, dateRule) != (topic == "date_select") {
			t.Errorf("date-only source scope leaked or disappeared for %s", topic)
		}
		if topic == "date_select" {
			for _, required := range []string{"不以局部片段泛称九星全部", "报告自有建议", "不得扩成普遍原文"} {
				if !strings.Contains(instruction, required) {
					t.Errorf("date writer missing source scope %q", required)
				}
			}
		}
	}
}

// This fixture verifies policy transport and repair bounds, not real model quality.
func TestReportGroundingPolicyIsSentToWriterCriticAndRepair(t *testing.T) {
	bad := reportBody{Summary: "象征显示用户已有安排能力；这是本地合成报告的待核对断言。", Content: "## 依据\n" + strings.Repeat("本地合成回归只验证指令传递，不代表真实报告质量。", 25) + "\n## 比较\n| 选项 | 行动 |\n| --- | --- |\n| 试听 | 先核对时间 |\n## 行动\n按已确认资料复盘。"}
	good := bad
	good.Summary = "若确有可支配时间，可先试听再作决定，具体安排仍需要本人确认。"
	writers, reviews := 0, 0
	p := provider.NewOpenAICompatible("http://report-grounding.invalid", "fixture", "fixture", "")
	p.SetHTTPClient(&http.Client{Transport: defaultContextTransport(func(r *http.Request) (*http.Response, error) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return nil, err
		}
		messages := req["messages"].([]any)
		system := fmt.Sprint(messages[0].(map[string]any)["content"])
		thinking, _ := req["thinking"].(map[string]any)
		wantEffort := "low"
		if isReportCalibrationFixtureRequest(req) {
			wantEffort = "high"
		}
		if req["max_tokens"] != float64(16384) || req["reasoning_effort"] != wantEffort || thinking["type"] != "enabled" {
			t.Error("grounding rules changed default budget or thinking policy")
		}
		w := httptest.NewRecorder()
		if isReportCalibrationFixtureRequest(req) {
			reviews++
			for _, required := range []string{"现实情况只据用户已确认输入", "已有或已发生仍是事实断言", "引号与编号之间隔有提示或解释语", "同句或同段"} {
				if !strings.Contains(system, required) {
					t.Errorf("critic missing boundary %q", required)
				}
			}
			var input reportCalibrationInput
			if err := json.Unmarshal([]byte(fmt.Sprint(messages[1].(map[string]any)["content"])), &input); err != nil {
				return nil, err
			}
			want := bad
			if reviews == 2 {
				want = good
			}
			if input.Draft != want || string(input.Inputs) != `{}` {
				t.Error("critic did not receive the exact current draft/confirmed inputs")
			}
			if reviews == 1 {
				writeReportCalibrationFixture(w, `{"issues":[{"quote":"显示用户已有安排能力","basis":"输入没有提供实际时间安排能力，牌卦象意不能确认现实情况。"}]}`)
			} else {
				writeReportCalibrationFixture(w, `{"issues":[]}`)
			}
		} else {
			writers++
			for _, required := range []string{"现实情况只据用户已确认输入", "优先作明确标为释义或解释的转述"} {
				if !strings.Contains(system, required) {
					t.Errorf("writer/repair missing boundary %q", required)
				}
			}
			body := bad
			if writers == 2 {
				body = good
				last := fmt.Sprint(messages[len(messages)-1].(map[string]any)["content"])
				if !strings.Contains(last, reportCitationWritingRules) || !strings.Contains(last, "不复述诊断") {
					t.Error("repair request lost final source numbering/diagnostic boundary")
				}
			}
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
	got, doc, usage, err := executeReport(context.Background(), s, &Report{ID: 1, SkillCode: "fixture-topic", Question: "合成指令传递测试", InputsJSON: `{}`, ChaptersJSON: `["依据","比较","行动"]`})
	if err != nil || got != good || writers != 2 || reviews != 2 || doc.ModelCalls != 4 || doc.ToolCalls != 0 || usage.PromptTokens != 28 || usage.CompletionTokens != 18 || usage.ReasoningTokens != 8 {
		t.Fatalf("changed repair bounds/draft/usage: err=%v writers=%d reviews=%d models=%d tools=%d bodyEqual=%t usage=%+v", err, writers, reviews, doc.ModelCalls, doc.ToolCalls, got == good, usage)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
