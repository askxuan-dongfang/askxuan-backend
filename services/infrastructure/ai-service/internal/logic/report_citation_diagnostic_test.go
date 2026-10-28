package logic

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/reportdoc"
)

func TestCitationFormatDiagnosticUsesVisibleFieldAndPosition(t *testing.T) {
	doc := reportdoc.New()
	doc.Blocks = []reportdoc.Block{{Kind: "citations", Items: []reportdoc.Item{{Label: "资料1", Value: "忠實原文"}}}}
	const prefix = "序言[资料1]链接[资料1]之后【"
	body := reportBody{Content: "序言[资料1]链接[资料1](https://example.invalid/(資料９９))之后【资料2】再有（资料3）"}
	err := validateReportCitations(body, doc)
	wantPosition := fmt.Sprintf("content可见文字第%d字", utf8.RuneCountInString(prefix)+1)
	if err == nil || !strings.Contains(err.Error(), wantPosition) || !strings.Contains(err.Error(), "【资料2】") || strings.Contains(err.Error(), "資料９９") {
		t.Fatalf("first invalid marker was not located in visible content: %v, want %s", err, wantPosition)
	}
	if !strings.Contains(err.Error(), "引用编号必须使用[资料N]") {
		t.Fatal("format repair lost its canonical instruction", err)
	}
	if err := validateReportCitations(reportBody{Content: "有效[资料1](https://example.invalid/(資料９９))"}, doc); err != nil {
		t.Fatal("a link target was treated as visible citation text", err)
	}
}

func TestCitationFormatDiagnosticContextIsBounded(t *testing.T) {
	prefix := "PRIVATE_PREFIX_OUTSIDE_CONTEXT" + strings.Repeat("甲", 80) + "（"
	err := validateReportCitations(reportBody{Summary: prefix + "資料２）" + strings.Repeat("乙", 80) + "PRIVATE_SUFFIX_OUTSIDE_CONTEXT"}, reportdoc.New())
	wantPosition := fmt.Sprintf("summary可见文字第%d字", utf8.RuneCountInString(prefix)+1)
	if err == nil || !strings.Contains(err.Error(), wantPosition) || strings.Contains(err.Error(), "PRIVATE_") {
		t.Fatalf("wrong field/position or unbounded context: %v", err)
	}
	_, quoted, ok := strings.Cut(err.Error(), "附近 ")
	if !ok {
		t.Fatal("missing local context", err)
	}
	quoted, _, ok = strings.Cut(quoted, "：引用编号")
	context, unquoteErr := strconv.Unquote(quoted)
	if !ok || unquoteErr != nil || utf8.RuneCountInString(context) > 40 || !strings.Contains(context, "（資料２）") {
		t.Fatalf("local context must identify the marker in at most 40 runes: %q %v", context, unquoteErr)
	}
}

func TestCitationDiagnosticKeepsUnknownAndDistortedQuotesRejected(t *testing.T) {
	doc := reportdoc.New()
	doc.Blocks = []reportdoc.Block{{Kind: "citations", Items: []reportdoc.Item{{Label: "资料1", Value: "乙不栽植，千株不長。"}}}}
	for _, content := range []string{"未知[资料99]", "“千株不长”[资料1]", "“千株不長”【资料1】"} {
		if err := validateReportCitations(reportBody{Content: content}, doc); err == nil {
			t.Fatalf("strict citation gate weakened: %s", content)
		}
	}
	if err := validateReportCitations(reportBody{Content: "“千株不長”[资料1]"}, doc); err != nil {
		t.Fatal(err)
	}
}

func TestZiweiWritingKeepsTransitCitationConditionsSeparate(t *testing.T) {
	instruction := reportInstruction(&Report{SkillCode: "ziwei"}, &model.AISkill{})
	for _, required := range []string{"本命、大限、小限、流年", "大小二限遇某星", "流年某星化忌不等于大小二限遇该星", "缺少条件时不套用该引文"} {
		if !strings.Contains(instruction, required) {
			t.Fatalf("missing Ziwei citation condition: %s", required)
		}
	}
	for _, topic := range []string{"bazi", "marriage", "liuyao", "dream", "tarot"} {
		if strings.Contains(reportInstruction(&Report{SkillCode: topic}, &model.AISkill{}), "大小二限遇某星") {
			t.Fatalf("Ziwei-only conditions leaked into %s", topic)
		}
	}
	const quote = "大小二限遇廉贞，更有天刑忌刃侵"
	issues, err := parseReportCalibration(`{"issues":[{"quote":"`+quote+`","basis":"草稿只列流年廉贞化忌，没有相应大小限节点满足引文条件的证据。"}]}`, reportBody{Content: quote})
	if err != nil || len(issues) != 1 || issues[0].Quote != quote {
		t.Fatal("grounded citation-condition issue was lost or treated as a pass", issues, err)
	}
}

func TestFengshuiWritingRequiresSourcesForDirectionalRestrictions(t *testing.T) {
	instruction := reportInstruction(&Report{SkillCode: "fengshui"}, &model.AISkill{})
	for _, required := range []string{"缺少对应本轮原文时", "不据星数建议回避方位或限制现实活动", "条件句也不能替代文献依据", "现场动线和眩光的对照验证"} {
		if !strings.Contains(instruction, required) {
			t.Fatalf("missing source boundary or practical verification: %s", required)
		}
	}
}

func TestDateWritingUsesReadableRangeAndFaithfulMethodParaphrases(t *testing.T) {
	instruction := reportInstruction(&Report{SkillCode: "date_select"}, &model.AISkill{})
	if strings.Contains(instruction, "weekdayOnly") {
		t.Fatal("date writing examples repeat the internal field they must avoid")
	}
	for _, required := range []string{"是否包含周末与周内", "星期不等于法定假日或调休", "明确标为释义或解释的转述", "不用引号把转述冒充原句", "只讨论本次实际日期和对应宜忌项", "未列出不等于忌"} {
		if !strings.Contains(instruction, required) {
			t.Fatalf("missing date method/status boundary: %s", required)
		}
	}
}
