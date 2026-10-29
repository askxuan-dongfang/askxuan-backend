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

func TestReportTarotObjectFieldIsRejectedOnlyWhenVisible(t *testing.T) {
	const observed96 = "本次为三牌阵单对象读牌（对象字段为空，不区分「你／对方」）"
	for _, tc := range []struct {
		name, tool, text string
		wantErr          bool
	}{
		{"observed_96", "tarot", observed96, true},
		{"visible_link_label", "tarot", "参见[对象字段](https://example.invalid/reading)。", true},
		{"personal_reading", "tarot", "本次仅为你本人读牌，不区分你和对方。", false},
		{"link_target", "tarot", "参见[说明](https://example.invalid/对象字段)。", false},
		{"other_field", "tarot", "用户已确认课程名称和时间范围，其他安排待确认。", false},
		{"technical_question_other_topic", "dream", "技术问题是对象字段为空，需由项目维护者核对。", false},
		{"unrelated_evidence", "search_knowledge", observed96, false},
		{"tool_name_substring", "other_tarot", observed96, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := reportdoc.New()
			doc.Evidence = []reportdoc.Evidence{{Tool: tc.tool, Text: `{}`}}
			for _, field := range []string{"summary", "content"} {
				t.Run(field, func(t *testing.T) {
					body := reportBody{}
					if field == "summary" {
						body.Summary = tc.text
					} else {
						body.Content = tc.text
					}
					if err := validateReportImplementationDetails(body, doc); (err != nil) != tc.wantErr {
						t.Fatalf("visible detail check=%v, wantErr=%t", err, tc.wantErr)
					}
				})
			}
		})
	}
}

var reportTarotEmotionRules = []string{"牌义不能判定用户当前情绪或顾虑成因", "摘要不作这类判断", "二分顾虑或排除能力不足", "情绪归因只能写成本人验证的问题或条件"}

func TestReportTarotPositionAndSummaryGroundingRulesStayTopicScoped(t *testing.T) {
	tarotOnly := []string{
		"过去、现在、未来仅为牌阵位置",
		"不能据此断言真实过往能力、资源或稳定生活秩序",
		"摘要只复述用户已确认的现实事实并列行动选项",
		"牌义写为象征、条件或自问，不排除其他可能顾虑",
		"仅为你本人读牌",
		"不展示对象字段或空值",
	}
	tarotOnly = append(tarotOnly, reportTarotEmotionRules...)
	for _, topic := range []string{"bazi", "marriage", "fengshui", "ziwei", "qimen", "tarot", "liuyao", "naming", "dream", "date_select", "fortune", "face_palm"} {
		t.Run(topic, func(t *testing.T) {
			instruction := reportInstruction(&Report{SkillCode: topic}, &model.AISkill{})
			critic, err := reportCalibrationRequest(provider.Request{MaxTokens: 8192}, 32768, reportCalibrationInput{Topic: topic})
			if err != nil {
				t.Fatal(err)
			}
			for _, prompt := range []string{instruction, critic.SystemPrompt} {
				for _, rule := range tarotOnly {
					if strings.Contains(prompt, rule) != (topic == "tarot") {
						t.Errorf("tarot-only rule missing or leaked to %s: %q", topic, rule)
					}
				}
			}
			if topic == "tarot" {
				for _, preserved := range []string{"牌义不能证明用户的收入、支付能力、经历或性格", "课程三个月不等于十二次课"} {
					if !strings.Contains(instruction, preserved) {
						t.Errorf("existing tarot boundary lost: %q", preserved)
					}
				}
			}
		})
	}
}

// These short issues come from the synthetic report 96 audit, not a model fixture pass.
func TestReportTarotObserved96IssuesKeepStrictLiteralGrounding(t *testing.T) {
	observed := []reportCalibrationIssue{
		{
			Quote: "塔罗原典把塔罗视为象征体系，符号如同可无限组合的字母表，能指向深层含义[资料2]",
			Basis: "资料2原文为“真正的塔罗本质是象征体系，其符号如同可无限组合的字母表，能指向神秘学奥义”；草稿将“神秘学奥义”改写为“深层含义”且未明示改写，属于实际改字。",
		},
		{
			Quote: "本次为三牌阵单对象读牌（对象字段为空，不区分「你／对方」）",
			Basis: "正文展示了内部字段名“对象字段”及原始空值转写。按规则，摘要和正文不应展示机器字段名或枚举值，应写成日常中文，例如“仅为你个人读牌”，而不是出现“对象字段为空”。本次计算中对象字段确为空，但正文不应出现该字段名。",
		},
		{
			Quote: "你的顾虑不是「能不能学」，而是「在已有稳定节奏上再加三个月，值不值得、会不会失衡」。这组基调偏稳健：你在意的是守住已建立的生活秩序，而非能力不足。",
			Basis: "用户仅确认“有兴趣但担心工作忙”，未确认具体顾虑成因。此句排除“能不能学”并归因于“已有稳定节奏上再加三个月”“守住已建立的生活秩序”，属于未确认的心理成因断言，且排除了其他可能顾虑。按规则应写为明确条件、反思问题或待确认项，而非直接断言。",
		},
	}
	for _, issue := range observed {
		raw, err := json.Marshal(map[string]any{"issues": []reportCalibrationIssue{issue}})
		if err != nil {
			t.Fatal(err)
		}
		for _, body := range []reportBody{{Summary: issue.Quote}, {Content: issue.Quote}} {
			issues, err := parseReportCalibration(string(raw), body)
			if err != nil || len(issues) != 1 || issues[0] != issue {
				t.Fatalf("actual issue was discarded or rewritten: issues=%+v err=%v", issues, err)
			}
			changed := issue
			changed.Quote = "草稿未出现的改写：" + issue.Quote
			ungrounded, err := json.Marshal(map[string]any{"issues": []reportCalibrationIssue{changed}})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := parseReportCalibration(string(ungrounded), body); err == nil {
				t.Fatal("accepted an issue whose literal quote is absent from the draft")
			}
		}
	}
	body := reportBody{Content: observed[1].Quote}
	for _, raw := range []string{
		`{"issues":null}`,
		`{"issues":[],"passed":true}`,
		`{"issues":[{"quote":"对象字段为空","basis":"本轮实际问题","extra":"unknown"}]}`,
		`{"issues":[{"quote":"對象字段為空","basis":"繁简与字形不能替代原文定位"}]}`,
	} {
		if _, err := parseReportCalibration(raw, body); err == nil {
			t.Fatalf("accepted invalid or nonliteral audit: %s", raw)
		}
	}
}

// Observed report99 claims test shared rules and the existing one-repair path;
// synthetic responses do not establish real-model quality or draw new cards.
func TestReportTarotObserved99EmotionIssuesReachWriterCriticAndSingleRepair(t *testing.T) {
	const question = "我在考虑报名持续三个月的周末插画课程，有兴趣但担心工作忙。请比较先试听和直接报名。"
	const inputs = `{"spread":"three"}`
	const summaryClaim = "当前顾虑偏情绪化"
	const contentClaim = "结合你的实际情况（有兴趣、担心工作忙），这更接近一种“想投入又怕坚持不下来”的忐忑，而不是能力不足。"
	bad := reportBody{Summary: "你有兴趣并担心工作忙。" + summaryClaim + "；可比较试听与直接报名的安排。", Content: "## 已确认情况\n" + contentClaim + "\n" + strings.Repeat("本地合成回归只验证指令传递，不代表真实模型质量。", 15) + "\n## 比较\n|选项|核对|\n|---|---|\n|试听|确认课程规则|\n## 行动\n先核对课次和时间。"}
	good := bad
	good.Summary = "你对课程有兴趣并担心工作忙。可以先核对课次、时段和试听规则，再决定。"
	good.Content = strings.Replace(bad.Content, contentClaim, "可以自问：除了工作安排，是否还有精力、费用、学习基础或情绪方面的顾虑？哪些相符需要本人确认，牌义不能替本人归因或排除能力问题。", 1)
	issues := []reportCalibrationIssue{{Quote: summaryClaim, Basis: "用户只确认兴趣及工作忙，未确认顾虑偏情绪化，牌义不能确定实际情绪或顾虑成因。"}, {Quote: contentClaim, Basis: "兴趣或工作忙不能排除未确认的能力顾虑；总免责声明或后续自问不撤回这项现实归因及排他判断。"}}
	native := `{"问卜设定":{"牌阵":"三牌阵","随机种子":"synthetic-fixed-native-seed"},"牌阵展开":[{"位置":"过去","塔罗牌":"宝剑五","状态":"逆位","元素":"风","核心基调":["怨恨不散","渴望和解","学会宽恕"]},{"位置":"现在","塔罗牌":"圣杯侍从","状态":"逆位","元素":"水","核心基调":["情感不成熟","不安全感","失望沮丧"]},{"位置":"未来","塔罗牌":"节制","状态":"正位","元素":"火","星象":"射手座","核心基调":["平衡","耐心","调和"]}]}`
	mcpCalls := 0
	mcp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mcpCalls++
		var call struct {
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&call); err != nil {
			t.Error(err)
		}
		if call.Params.Name != "tarot" || call.Params.Arguments["spreadType"] != "three-card" {
			t.Error("native fixture lost selected spread")
		}
		if _, ok := call.Params.Arguments["seed"]; ok {
			t.Error("writer injected a native seed")
		}
		fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"result":{"structuredContent":%s}}`, native)
	}))
	t.Cleanup(mcp.Close)
	writers, reviews := 0, 0
	p := provider.NewOpenAICompatible("http://tarot-grounding.invalid", "fixture", "fixture", "")
	p.SetHTTPClient(&http.Client{Transport: defaultContextTransport(func(r *http.Request) (*http.Response, error) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return nil, err
		}
		messages := req["messages"].([]any)
		for _, rule := range reportTarotEmotionRules {
			if !strings.Contains(fmt.Sprint(messages[0].(map[string]any)["content"]), rule) {
				t.Errorf("writer, critic or repair lost emotion boundary: %q", rule)
			}
		}
		critic := isReportCalibrationFixtureRequest(req)
		effort := "low"
		if critic {
			effort = "high"
		}
		thinking, _ := req["thinking"].(map[string]any)
		if req["max_tokens"] != float64(16384) || req["reasoning_effort"] != effort || thinking["type"] != "enabled" {
			t.Error("emotion rules changed output or thinking budget")
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
			if input.Topic != "tarot" || input.Question != question || string(input.Inputs) != inputs || input.Draft != want || len(input.Charts) != 1 || !strings.Contains(string(input.Calculations), "synthetic-fixed-native-seed") {
				t.Error("critic lost known inputs, exact draft or original native calculation")
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
						t.Error("single repair lost exact observed issue")
					}
				}
				if !strings.Contains(last, "不要重新计算") {
					t.Error("single repair lost native draw preservation")
				}
			}
			raw, _ := json.Marshal(body)
			event, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "content": string(raw)}, "finish_reason": "stop"}}, "usage": map[string]any{"prompt_tokens": 3, "completion_tokens": 2, "completion_tokens_details": map[string]any{"reasoning_tokens": 1}}})
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", event)
		}
		response := w.Result()
		response.Request = r
		return response, nil
	})})
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	for _, stage := range []string{"checking", "calculating", "writing"} {
		mock.ExpectExec("UPDATE ai_report SET generation_stage").WithArgs(stage, int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	}
	skill := &model.AISkill{Code: "tarot", Status: model.SkillStatusEnabled, InputSchema: reportToolSchema("tarot"), ToolConfig: `{"enabled":true,"server":"taibu","tool":"tarot"}`}
	s := &svc.ServiceContext{DB: sqlx.NewSqlConnFromDB(db), SkillModel: reportSkills{skill}, Provider: p, MCP: agent.NewMCPClient(true, mcp.URL, 10), Guard: agent.NewGuard(20000, nil)}
	s.AIConfig.ContextWindow = 1048576
	got, doc, usage, err := executeReport(context.Background(), s, &Report{ID: 1, SkillCode: "tarot", Question: question, InputsJSON: inputs, ChaptersJSON: `["已确认情况","比较","行动"]`})
	if err != nil || got != good || writers != 2 || reviews != 2 || doc.ModelCalls != 4 || mcpCalls != 1 || doc.ToolCalls != 1 || usage.PromptTokens != 28 || usage.CompletionTokens != 18 || usage.ReasoningTokens != 8 {
		t.Fatalf("changed repair/draw/usage bounds: err=%v bodyEqual=%t writers=%d reviews=%d models=%d mcp=%d tools=%d usage=%+v", err, got == good, writers, reviews, doc.ModelCalls, mcpCalls, doc.ToolCalls, usage)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
