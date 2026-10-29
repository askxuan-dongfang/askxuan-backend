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

var reportFengshuiLayoutRules = []string{
	"轮廓、房间功能及一处占道，不能推出家具摆位、路线数量或共享、物件与通道的空间关系",
	"已确认情况只复述用户观察；未提供的布局关系须列为待现场核对，不夹入已确认段",
}

func TestReportFengshuiLayoutRulesStayTopicScoped(t *testing.T) {
	for _, topic := range []string{"bazi", "marriage", "fengshui", "ziwei", "qimen", "tarot", "liuyao", "naming", "dream", "date_select", "fortune", "face_palm"} {
		critic, err := reportCalibrationRequest(provider.Request{MaxTokens: 8192}, 32768, reportCalibrationInput{Topic: topic})
		if err != nil {
			t.Fatal(err)
		}
		for _, prompt := range []string{reportInstruction(&Report{SkillCode: topic}, &model.AISkill{}), critic.SystemPrompt} {
			for _, rule := range reportFengshuiLayoutRules {
				if strings.Contains(prompt, rule) != (topic == "fengshui") {
					t.Errorf("layout rule missing or leaked to %s: %q", topic, rule)
				}
			}
		}
	}
}

// The short claim is from report92; local models verify shared policy transport
// and existing repair bounds, not actual model interpretation or content quality.
func TestReportFengshuiLayoutIssueReachesWriterCriticAndSingleRepair(t *testing.T) {
	const question = "矩形单间兼作客厅与工作间，已知纸箱占道，如何核对动线？"
	const inputs = `{"scene":"home","circulation":"blocked"}`
	const claim = "家具与纸箱共用同一条主要动线"
	bad := reportBody{Summary: "已知纸箱占道；请先整理可移动杂物，再核实家具与通道的实际关系。", Content: "## 已确认情况\n" + claim + "。\n" + strings.Repeat("本地合成回归只验证指令传递，不代表真实模型质量。", 15) + "\n## 比较\n| 选项 | 检查 |\n| --- | --- |\n| 整理 | 复核净宽 |\n## 行动\n家具尺寸和实际摆位需现场核实。"}
	good := bad
	good.Content = strings.Replace(bad.Content, claim, "纸箱占道已确认；家具是否影响通行、客厅与工作区是否共用路线，需核对实际布局", 1)
	issue := reportCalibrationIssue{Quote: claim, Basis: "用户只确认纸箱占道与单间用途，未确认家具位置、路线数量或共享关系；后文待实测没有撤回这句现状断言。"}
	mcpCalls := 0
	mcp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mcpCalls++
		fmt.Fprint(w, `{"jsonrpc":"2.0","id":1,"result":{"structuredContent":{"version":"20260929.1","charts":[{"kind":"table","title":"空间尺寸","columns":["宽m","长m"],"rows":[["4.00","3.00"]]}]}}}`)
	}))
	t.Cleanup(mcp.Close)
	writers, reviews := 0, 0
	p := provider.NewOpenAICompatible("http://fengshui-grounding.invalid", "fixture", "fixture", "")
	p.SetHTTPClient(&http.Client{Transport: defaultContextTransport(func(r *http.Request) (*http.Response, error) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return nil, err
		}
		messages := req["messages"].([]any)
		for _, rule := range reportFengshuiLayoutRules {
			if !strings.Contains(fmt.Sprint(messages[0].(map[string]any)["content"]), rule) {
				t.Errorf("writer, critic or repair lost layout rule: %q", rule)
			}
		}
		critic := isReportCalibrationFixtureRequest(req)
		effort := "low"
		if critic {
			effort = "high"
		}
		if req["max_tokens"] != float64(16384) || req["reasoning_effort"] != effort {
			t.Error("layout policy changed output or reasoning budget")
		}
		w := httptest.NewRecorder()
		if critic {
			reviews++
			var input reportCalibrationInput
			if err := json.Unmarshal([]byte(fmt.Sprint(messages[1].(map[string]any)["content"])), &input); err != nil {
				return nil, err
			}
			want := bad
			if reviews == 2 {
				want = good
			}
			if input.Topic != "fengshui" || input.Question != question || string(input.Inputs) != `{"circulation":"blocked","scene":"home"}` || input.Draft != want || len(input.Charts) != 1 || !strings.Contains(string(input.Calculations), "4.00") {
				t.Error("critic lost current draft, known inputs or original calculation")
			}
			result := `{"issues":[]}`
			if reviews == 1 {
				raw, _ := json.Marshal(map[string]any{"issues": []reportCalibrationIssue{issue}})
				result = string(raw)
			}
			writeReportCalibrationFixture(w, result)
		} else {
			writers++
			body := bad
			if writers == 2 {
				body = good
				last := fmt.Sprint(messages[len(messages)-1].(map[string]any)["content"])
				if !strings.Contains(last, claim) || !strings.Contains(last, "不要重新计算") {
					t.Error("repair lost exact layout issue or cached calculation boundary")
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
	skill := &model.AISkill{Code: "fengshui", Status: model.SkillStatusEnabled, InputSchema: reportToolSchema("fengshui"), ToolConfig: `{"enabled":true,"server":"taibu","tool":"fengshui"}`}
	s := &svc.ServiceContext{DB: sqlx.NewSqlConnFromDB(db), SkillModel: reportSkills{skill}, Provider: p, MCP: agent.NewMCPClient(true, mcp.URL, 10), Guard: agent.NewGuard(20000, nil)}
	s.AIConfig.ContextWindow = 1048576
	got, doc, usage, err := executeReport(context.Background(), s, &Report{ID: 1, SkillCode: "fengshui", Question: question, InputsJSON: inputs, ChaptersJSON: `["已确认情况","比较","行动"]`})
	if err != nil || got != good || writers != 2 || reviews != 2 || doc.ModelCalls != 4 || mcpCalls != 1 || doc.ToolCalls != 1 || usage.PromptTokens != 28 || usage.CompletionTokens != 18 || usage.ReasoningTokens != 8 {
		t.Fatalf("changed repair/calculation/usage bounds: err=%v bodyEqual=%t writers=%d reviews=%d models=%d mcp=%d tools=%d usage=%+v", err, got == good, writers, reviews, doc.ModelCalls, mcpCalls, doc.ToolCalls, usage)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
