package logic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/reportdoc"
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

func TestReportTarotPositionAndSummaryGroundingRulesStayTopicScoped(t *testing.T) {
	tarotOnly := []string{
		"过去、现在、未来仅为牌阵位置",
		"不能据此断言真实过往能力、资源或稳定生活秩序",
		"摘要只复述用户已确认的现实事实并列行动选项",
		"牌义写为象征、条件或自问，不排除其他可能顾虑",
		"仅为你本人读牌",
		"不展示对象字段或空值",
	}
	for _, topic := range []string{"bazi", "marriage", "fengshui", "ziwei", "qimen", "tarot", "liuyao", "naming", "dream", "date_select", "fortune", "face_palm"} {
		t.Run(topic, func(t *testing.T) {
			instruction := reportInstruction(&Report{SkillCode: topic}, &model.AISkill{})
			for _, rule := range tarotOnly {
				if strings.Contains(instruction, rule) != (topic == "tarot") {
					t.Errorf("tarot-only rule missing or leaked to %s: %q", topic, rule)
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
