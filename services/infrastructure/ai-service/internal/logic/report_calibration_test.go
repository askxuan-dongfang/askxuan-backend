package logic

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/reportdoc"
)

func TestCalibrationRequiresGroundedExplicitResult(t *testing.T) {
	body := reportBody{Summary: "对方伤官透于时", Content: "你的木仅藏于亥中甲。你偏临场，对方偏计划。"}
	for _, raw := range []string{
		`{"issues":[]}`,
		`{"issues":[{"quote":"伤官透于时","basis":"对方时柱戊子，天干为偏印；伤官在子中藏干"}]}`,
	} {
		if _, err := parseReportCalibration(raw, body); err != nil {
			t.Fatal(raw, err)
		}
	}
	tooMany, _ := json.Marshal(map[string]any{"issues": make([]reportCalibrationIssue, 9)})
	longQuote, _ := json.Marshal(map[string]any{"issues": []reportCalibrationIssue{{Quote: strings.Repeat("字", 201), Basis: "依据"}}})
	longBasis, _ := json.Marshal(map[string]any{"issues": []reportCalibrationIssue{{Quote: "伤官透于时", Basis: strings.Repeat("字", 501)}}})
	for _, raw := range []string{
		``, `null`, `{}`, `{"issues":null}`, `{"issues":"通过"}`, `{"issues":[],"passed":true}`,
		`{"issues":[{"quote":"正文没有这句话","basis":"依据"}]}`,
		`{"issues":[{"quote":"伤官透于时","basis":""}]}`,
		`{"issues":[{"quote":"","basis":"依据"}]}`,
		`{"issues":[{"quote":"伤官透于时","basis":"依据","extra":"未知"}]}`,
		`{"issues":[{"quote":"伤官透于时","basis":"冲突"}],"issues":[]}`, `{"issues":[]} {"issues":[]}`, "```json\n{\"issues\":[]}\n```", string(tooMany), string(longQuote), string(longBasis),
	} {
		if _, err := parseReportCalibration(raw, body); err == nil {
			t.Fatalf("accepted unverifiable review: %s", raw)
		}
	}
}

func TestCalibrationRetainsEvidenceAndFailsInsteadOfTruncating(t *testing.T) {
	doc := reportdoc.New()
	doc.Blocks = []reportdoc.Block{{Kind: "citations", Items: []reportdoc.Item{{Label: "资料7", Value: "本次实际检索片段", Detail: "原文位置"}}}}
	doc.Evidence = []reportdoc.Evidence{{Tool: "recall_memory", Text: "PRIVATE MEMORY"}, {Tool: "search_knowledge", Text: "资料7：本次实际检索片段"}, {Tool: "participant", Text: "duplicate chart"}}
	in := reportCalibrationInput{Topic: "marriage", Question: "一个计划一个临场", Inputs: json.RawMessage(`{"birthplace":"沙依巴克区","partnerBirthplace":"西湖区"}`), Calculations: json.RawMessage(`[{"对象":"你","四柱":"乙巳"},{"对象":"对方","四柱":"戊子"}]`), Sources: reportCalibrationSources(doc), Draft: reportBody{Summary: "待核对摘要", Content: "待核对正文"}}
	req, err := reportCalibrationRequest(provider.Request{Model: "selected-model", MaxTokens: 8192, ThinkingEnabled: true, ReasoningEffort: "high"}, 32768, in)
	if err != nil || req.Model != "selected-model" || req.MaxTokens != 8192 || !req.JSONMode || !req.ThinkingEnabled || req.ReasoningEffort != "high" || len(req.Messages) != 1 {
		t.Fatal(req, err)
	}
	for _, required := range []string{"沙依巴克区", "西湖区", "乙巳", "戊子", "资料7"} {
		if !strings.Contains(req.Messages[0].Content, required) {
			t.Fatalf("lost participant/source: %s", required)
		}
	}
	if strings.Contains(req.Messages[0].Content, "PRIVATE MEMORY") || strings.Contains(req.Messages[0].Content, "duplicate chart") {
		t.Fatal("review included private or duplicate evidence")
	}
	in.Draft.Content = strings.Repeat("报告", 32768)
	if _, err = reportCalibrationRequest(req, 32768, in); err == nil {
		t.Fatal("oversized review silently truncated or sent")
	}
}

func TestReportRoleInstructionsOnlyApplyToMarriage(t *testing.T) {
	for _, code := range []string{"bazi", "fortune", "qimen", "ziwei"} {
		instruction := reportInstruction(&Report{SkillCode: code}, &model.AISkill{})
		if strings.Contains(instruction, "计划方") || strings.Contains(instruction, "临场方") {
			t.Fatalf("unrelated role instructions leaked into %s", code)
		}
	}
	if !strings.Contains(reportInstruction(&Report{SkillCode: "marriage"}, &model.AISkill{}), "计划方") {
		t.Fatal("marriage lost role calibration")
	}
}
