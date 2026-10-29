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

var reportLiuyaoObserved95Rules = []string{
	"兄弟旺不证明团队已具实力",
	"父母日破不证明实际旧流程存在漏洞",
	"世爻空亡不证明当前兜底未就绪",
	"空亡、日破、动变等只描述传统卦象倾向，不能证明任何对象的现实实施或完成状态",
	"项目、方案或流程是否已启动、落地、验收，须从用户确认或实际记录核实",
	"先列已确认现实条件，再把象意单列为传统解释，不用提示或说明当前把二者连接为现状结论",
	"不得从爻辞推断真实合作关系、他人动机或猜疑成因",
	"摘要和正文不能把这些象意写成用户已确认的现状，应改为条件或待核对检查项",
	"总免责声明不能抵消具体断言",
	"灰度、回滚可作为独立建议，不声称实际现状",
}

func TestReportLiuyaoRealWorldGroundingRulesStayTopicScoped(t *testing.T) {
	for _, topic := range []string{"bazi", "marriage", "fengshui", "ziwei", "qimen", "tarot", "liuyao", "naming", "dream", "date_select", "fortune", "face_palm"} {
		t.Run(topic, func(t *testing.T) {
			writer := reportInstruction(&Report{SkillCode: topic}, &model.AISkill{})
			critic, err := reportCalibrationRequest(provider.Request{MaxTokens: 8192}, 32768, reportCalibrationInput{Topic: topic, Draft: reportBody{Summary: "只核对本次已确认的现实事实。"}})
			if err != nil {
				t.Fatal(err)
			}
			for _, rule := range reportLiuyaoObserved95Rules {
				for _, prompt := range []string{writer, critic.SystemPrompt} {
					if strings.Contains(prompt, rule) != (topic == "liuyao") {
						t.Errorf("liuyao-only writer/critic rule missing or leaked to %s: %q", topic, rule)
					}
				}
			}
		})
	}
}

// The short claims are from synthetic reports 95 and 97. MCP and model are
// fixtures: this verifies policy transport and repair bounds, not model quality.
func TestReportLiuyaoObserved95ClaimsReachWriterCriticAndSingleRepair(t *testing.T) {
	const question = "现有模块先做小范围试用，还是一次替换全部旧流程？重点梳理团队协作、变更风险与可执行的验证步骤。"
	const inputs = `{"method":"time","eventTime":"2026-10-28T14:30","location":"中国新疆乌鲁木齐市沙依巴克区","timeBasis":"true_solar","yongShenTarget":"兄弟"}`
	bad := reportBody{
		Summary: "用神兄弟丑土旺相，团队有实力可用；世爻子孙空亡、应爻父母日破并存，提示协作摩擦与旧流程破口。",
		Content: "## 影响因素\n用神兄弟丑土旺相、静、不空，说明“同类之人”（团队成员、协作方）本身是有实力、可依靠的，具备推进试点的条件。猜疑多半是自我放大的，真实关系仍是协作，可通过沟通化解。\n" +
			"应爻初九父母巳火“日破”，指向对方环境或旧流程一侧有“破口”，一次性触碰全部旧流程，风险会叠加在这处破口上。世爻子孙酉金为“制鬼、解忧”之位，但当前值空亡，说明“化解压力、兜底保障”的能力此刻尚未到位。\n" +
			"## 方案比较\n| 选择 | 核对项目 |\n| --- | --- |\n| 小范围试用 | 核对回退步骤 |\n| 全量替换 | 核对覆盖范围 |\n" +
			"当前保障机制（世爻子孙空亡）与旧流程破口（应爻父母日破）尚未处理到位前，不宜一步到位。\n" +
			"世持子孙而空亡，提示**新方案/新模块本身目前处于“未落实、未填实”的状态**。\n" +
			"## 行动建议\n先记录模块范围与验证指标。卦象不是确定性的成败预言；模块规模、团队人数、验收指标、时间窗口等现实参数未包含在本次资料中，最终决策仍需结合实际测试结果与专业意见。",
	}
	good := reportBody{
		Summary: "团队能力、旧流程漏洞及兜底保障状态均未在用户资料中确认。可把它们列为待核对项，灰度与回滚作为独立行动选项。",
		Content: "## 依据与边界\n" + strings.Repeat("本地合成回归只验证指令传递，不代表真实模型质量。", 8) +
			"\n团队是否已有推进实力、旧流程是否存在漏洞、当前兜底是否就绪，都应以用户确认或实际测试记录为准。兄弟旺、父母日破和世爻空亡本身不能证明这些现实状态。新方案是否已经启动或落地，须从实际实施记录核实；空亡仅作传统卦象倾向解释。\n" +
			"## 条件比较\n| 条件 | 核对行动 |\n| --- | --- |\n| 若现有记录不能说明团队能力 | 先核对负责人和验收经验 |\n| 若尚无流程缺陷证据 | 先做差异测试 |\n| 若保障状态未知 | 核对监控与回退演练记录 |\n" +
			"## 独立行动建议\n可选择小范围验证并准备回滚，具体规模与时点待确认。这些是独立的执行建议，不意味着已有漏洞或当前保障不足；没有证据时不排除其他可行选择。",
	}
	issues := []reportCalibrationIssue{
		{Quote: "用神兄弟丑土旺相，团队有实力可用", Basis: "用户没有确认团队已有实力；兄弟旺不证明现实能力。"},
		{Quote: "旧流程一侧有“破口”", Basis: "用户没有确认旧流程存在漏洞；父母日破不能证明实际缺陷。"},
		{Quote: "“化解压力、兜底保障”的能力此刻尚未到位", Basis: "用户没有确认当前兜底未就绪；世爻空亡不能证明现实状态，总免责声明不能抵消该断言。"},
		{Quote: "猜疑多半是自我放大的，真实关系仍是协作", Basis: "用户没有确认真实合作关系、他人动机或猜疑成因，不能从爻辞推断这些现实情况。"},
		{Quote: "世持子孙而空亡，提示**新方案/新模块本身目前处于“未落实、未填实”的状态**", Basis: "用户未确认模块实施状态；空亡不能证明项目、方案或流程是否实际落地。"},
	}
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
		mcpCalls++
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{"structuredContent": calculations["liuyao"]}})
	}))
	t.Cleanup(mcp.Close)
	writers, reviews := 0, 0
	p := provider.NewOpenAICompatible("http://liuyao-grounding.invalid", "fixture", "fixture", "")
	p.SetHTTPClient(&http.Client{Transport: defaultContextTransport(func(r *http.Request) (*http.Response, error) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return nil, err
		}
		messages := req["messages"].([]any)
		system := fmt.Sprint(messages[0].(map[string]any)["content"])
		for _, rule := range reportLiuyaoObserved95Rules {
			if !strings.Contains(system, rule) {
				t.Errorf("writer, repair or critic lost liuyao grounding: %q", rule)
			}
		}
		critic := isReportCalibrationFixtureRequest(req)
		wantEffort := "low"
		if critic {
			wantEffort = "high"
		}
		thinking, _ := req["thinking"].(map[string]any)
		if req["max_tokens"] != float64(16384) || req["reasoning_effort"] != wantEffort || thinking["type"] != "enabled" {
			t.Error("grounding repair changed output budget or reasoning policy")
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
			if input.Topic != "liuyao" || input.Question != question || string(input.Inputs) != `{"eventTime":"2026-10-28T14:30","location":"中国新疆乌鲁木齐市沙依巴克区","method":"time","timeBasis":"true_solar","yongShenTarget":"兄弟"}` || input.Draft != want {
				t.Error("critic lost actual question, confirmed inputs or exact current draft")
			}
			result := `{"issues":[]}`
			if reviews == 1 {
				raw, _ := json.Marshal(map[string]any{"issues": issues})
				result = string(raw)
			}
			writeReportCalibrationFixture(w, result)
		} else {
			writers++
			body := bad
			if writers == 2 {
				body = good
				last := fmt.Sprint(messages[len(messages)-1].(map[string]any)["content"])
				for _, issue := range issues {
					if !strings.Contains(last, issue.Quote) {
						t.Errorf("single repair lost exact issue: %q", issue.Quote)
					}
				}
				if !strings.Contains(last, "不要重新计算") {
					t.Error("single repair lost cached calculation boundary")
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
	skill := &model.AISkill{Code: "liuyao", Status: model.SkillStatusEnabled, InputSchema: reportToolSchema("liuyao"), ToolConfig: `{"enabled":true,"server":"taibu","tool":"liuyao"}`}
	s := &svc.ServiceContext{DB: sqlx.NewSqlConnFromDB(db), SkillModel: reportSkills{skill}, Provider: p, MCP: agent.NewMCPClient(true, mcp.URL, 10), Guard: agent.NewGuard(20000, nil)}
	s.AIConfig.ContextWindow = 1048576
	got, doc, usage, err := executeReport(context.Background(), s, &Report{ID: 1, SkillCode: "liuyao", Question: question, InputsJSON: inputs, ChaptersJSON: `["依据与边界","条件比较","独立行动建议"]`})
	if err != nil || got != good || writers != 2 || reviews != 2 || doc.ModelCalls != 4 || mcpCalls != 1 || doc.ToolCalls != 1 || usage.PromptTokens != 28 || usage.CompletionTokens != 18 || usage.ReasoningTokens != 8 {
		t.Fatalf("changed shared repair/calculation/usage bounds: err=%v bodyEqual=%t writers=%d reviews=%d models=%d mcp=%d tools=%d usage=%+v", err, got == good, writers, reviews, doc.ModelCalls, mcpCalls, doc.ToolCalls, usage)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
