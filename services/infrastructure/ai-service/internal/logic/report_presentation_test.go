package logic

import (
	"testing"

	"github.com/askxuan/ai-service/internal/reportdoc"
)

func TestReportRejectsNoncanonicalCitationNumbers(t *testing.T) {
	doc := reportdoc.New()
	doc.Blocks = []reportdoc.Block{{Kind: "citations", Items: []reportdoc.Item{{Label: "资料4", Value: "昔者莊周夢爲胡蝶。"}}}}
	for _, text := range []string{
		"原文“昔者莊周夢爲胡蝶”（资料4）", "原文“昔者莊周夢爲胡蝶”(资料4)",
		"原文“昔者莊周夢爲胡蝶”【资料4】", "释义：依据资料4整理。", "（资料99）",
		"[资料4，资料99]", "[资料4 译文]", "[资料４]", "[資料4]", "（资料 4）",
		"[参见资料4](https://example.org)", `没有链接开括号](资料4)`, `\[普通文本](资料4)`,
	} {
		for _, body := range []reportBody{{Summary: text}, {Content: text}} {
			if validateReportCitations(body, doc) == nil {
				t.Fatalf("noncanonical citation escaped review: %q", text)
			}
		}
	}
}

func TestReportMarkdownCitationLabelsRemainStrict(t *testing.T) {
	doc := reportdoc.New()
	doc.Blocks = []reportdoc.Block{{Kind: "citations", Items: []reportdoc.Item{{Label: "资料4", Value: "昔者莊周夢爲胡蝶。"}}}}
	for _, tc := range []struct {
		text    string
		wantErr bool
	}{
		{"“昔者莊周夢爲胡蝶”[资料4]", false},
		{"“昔者莊周夢爲胡蝶”[资料4](https://example.org/chapter)", false},
		{"“昔者莊周夢爲胡蝶”[资料4](https://example.org/资料99/a(b(c)))", false},
		{`“昔者莊周夢爲胡蝶”[资料4](https://example.org "注)资料99")`, false},
		{"更多[普通来源](https://example.org/O'Reilly/资料99)。", false},
		{"更多[普通来源](https://example.org/[资料99])。", false},
		{"[参考[资料4]](https://example.org/资料99)", false},
		{"[资料99](https://example.org/资料4)", true},
		{"“昔者庄周梦为蝴蝶”[资料4](https://example.org)", true},
		{"[资料4](https://example.org) 仍不能用（资料99）。", true},
	} {
		if err := validateReportCitations(reportBody{Content: tc.text}, doc); (err != nil) != tc.wantErr {
			t.Fatalf("Markdown citation check=%v wantErr=%t text=%q", err, tc.wantErr, tc.text)
		}
	}
}

func TestReportImplementationDetailsBindOnlyActualToolEvidence(t *testing.T) {
	doc := reportdoc.New()
	doc.Evidence = []reportdoc.Evidence{{Tool: "dream", Text: `{"version":"20260929.1","title":"梦境原述与反思线索","source":"梦境原述与反思问题整理"}`}}
	for _, tc := range []struct {
		name, text string
		wantErr    bool
	}{
		{"report82", "依据本次计算《梦境原述与反思线索》（版本 20260929.1，状态：已计算），四类线索如下。", true},
		{"internal_version", "本次计算（版本：20260929.1），如下。", true},
		{"internal_status", "依据《梦境原述与反思线索》（状态：已计算），如下。", true},
		{"internal_status_without_title", "本次计算，状态：已计算。", true},
		{"internal_rule_version", "规则版本20260929.1。", true},
		{"formatted_internal_version", "依据**《梦境原述与反思线索》**（版本：`20260929.1`）。", true},
		{"formatted_internal_status", "依据《梦境原述与反思线索》**状态**：已计算。", true},
		{"internal_label", "依据current_calculation结果整理。", true},
		{"internal_label_case", "依据CURRENT_CALCULATION结果整理。", true},
		{"user_project_version", "用户项目版本20260929.1是已确认背景，本次计算用于整理四类线索。", false},
		{"book_version", "引用文献版本20260929.1已由用户确认。", false},
		{"project_status", "项目状态：已计算，下一步由用户确认。", false},
		{"different_version", "规则版本20260929.10。", false},
		{"ordinary_version", "本次版本更新后，请按周记录。", false},
		{"ordinary_fact", "本次按梦境原述、醒来感受、现实背景和重复情况整理。", false},
		{"url_not_visible_version", "参见[说明](https://example.org/current_calculation)。", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, body := range []reportBody{{Summary: tc.text}, {Content: tc.text}} {
				if err := validateReportImplementationDetails(body, doc); (err != nil) != tc.wantErr {
					t.Fatalf("detail check=%v wantErr=%t", err, tc.wantErr)
				}
			}
		})
	}
	for _, evidence := range []reportdoc.Evidence{
		{Tool: "search_knowledge", Text: `{"version":"20260929.1","title":"梦境原述与反思线索"}`},
		{Tool: "dream", Text: `{"title":"梦境原述与反思线索"}`},
	} {
		other := reportdoc.New()
		other.Evidence = []reportdoc.Evidence{evidence}
		if err := validateReportImplementationDetails(reportBody{Content: "依据《梦境原述与反思线索》（版本20260929.1）。"}, other); err != nil {
			t.Fatal("unrelated or missing evidence cannot identify an internal version", err)
		}
	}
}

func TestReportRejectsObservedRawFormValuesOnlyInTheirTopic(t *testing.T) {
	for _, tc := range []struct {
		tool, text string
		wantErr    bool
	}{
		{"dream", "重复情况为 repeated，次数未提供。", true},
		{"dream", "重复情况原始字段值为 `repeated`。", true},
		{"dream", "- **重复情况（原始字段值）**：repeated（平台梦境图表转写）。", true},
		{"dream", "| 重复情况 | repeated | 用户原述 |", true},
		{"dream", "recurrence=unknown，具体次数未确认。", true},
		{"dream", "重复情况为 once。", true},
		{"dream", "曾重复出现，次数未提供。", false},
		{"dream", "用户项目名为 repeated，近期压力待自行核对。", false},
		{"dream", "梦境原述：梦中我反复唱英文歌 once，醒后记得这句歌词。", false},
		{"dream", "参见[说明](https://example.org/recurrence=repeated)。", false},
		{"date_select", "周末与工作日均纳入（weekdayOnly=all）。", true},
		{"date_select", "用户项目名为weekdayOnly。", false},
		{"date_select", "包含周末与周内日期，不包含节假日或调休判断。", false},
		{"naming", "英文项目名为 repeated。", false},
	} {
		doc := reportdoc.New()
		doc.Evidence = []reportdoc.Evidence{{Tool: tc.tool, Text: `{}`}}
		for _, body := range []reportBody{{Summary: tc.text}, {Content: tc.text}} {
			if err := validateReportImplementationDetails(body, doc); (err != nil) != tc.wantErr {
				t.Fatalf("tool=%s text=%q err=%v wantErr=%t", tc.tool, tc.text, err, tc.wantErr)
			}
		}
	}
}
