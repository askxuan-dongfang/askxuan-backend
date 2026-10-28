package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestReportCalibrationDiagnosticClassifiesIssueWithoutPrivateText(t *testing.T) {
	const privateQuote = "PRIVATE_QUOTE_甲"
	const privateBasis = "PRIVATE_BASIS_不得泄露"
	body := reportBody{Summary: "PRIVATE_SUMMARY_" + privateQuote, Content: "PRIVATE_CONTENT_" + strings.Repeat("界", 201)}
	for _, tc := range []struct {
		kind  string
		issue reportCalibrationIssue
		runes int
	}{
		{"quote_empty", reportCalibrationIssue{Quote: "", Basis: privateBasis}, 0},
		{"basis_empty", reportCalibrationIssue{Quote: privateQuote, Basis: ""}, 0},
		{"quote_too_long", reportCalibrationIssue{Quote: strings.Repeat("界", 201), Basis: privateBasis}, 201},
		{"basis_too_long", reportCalibrationIssue{Quote: privateQuote, Basis: strings.Repeat("界", 501)}, 501},
		{"quote_not_literal", reportCalibrationIssue{Quote: "PRIVATE_QUOTE_错字", Basis: privateBasis}, 16},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			for _, index := range []int{1, 2} {
				issues := []reportCalibrationIssue{}
				if index == 2 {
					issues = append(issues, reportCalibrationIssue{Quote: privateQuote, Basis: privateBasis})
				}
				issues = append(issues, tc.issue)
				raw, _ := json.Marshal(map[string]any{"issues": issues})
				got, err := parseReportCalibration(string(raw), body)
				want := fmt.Sprintf("ungrounded report calibration result: issue=%d reason=%s runes=%d", index, tc.kind, tc.runes)
				if err == nil || err.Error() != want || got != nil {
					t.Fatalf("issue diagnostic=%v want=%q returnedIssues=%d", err, want, len(got))
				}
				var diagnostic *reportCalibrationIssueError
				if !errors.As(err, &diagnostic) || diagnostic.Index != index || diagnostic.Kind != tc.kind || diagnostic.Runes != tc.runes {
					t.Fatal("typed issue position, category or rune count was lost")
				}
				for _, private := range []string{"PRIVATE_", privateQuote, privateBasis, "PRIVATE_SUMMARY", "PRIVATE_CONTENT", strings.Repeat("界", 10)} {
					if strings.Contains(err.Error(), private) {
						t.Fatal("issue diagnostic leaked draft, quote or basis text")
					}
				}
			}
		})
	}
	for _, tc := range []struct {
		issue reportCalibrationIssue
		kind  string
	}{
		{reportCalibrationIssue{Quote: " \n\t　", Basis: privateBasis}, "quote_empty"},
		{reportCalibrationIssue{Quote: privateQuote, Basis: " \n\t　"}, "basis_empty"},
	} {
		raw, _ := json.Marshal(map[string]any{"issues": []reportCalibrationIssue{tc.issue}})
		if _, err := parseReportCalibration(string(raw), body); err == nil || !strings.Contains(err.Error(), "issue=1 reason="+tc.kind+" runes=") {
			t.Fatalf("Unicode whitespace did not keep the strict empty-field gate: %v", err)
		}
	}
}

func TestReportCalibrationDiagnosticKeepsExactUnicodeAndLiteralBoundaries(t *testing.T) {
	for _, quote := range []string{strings.Repeat("界", 200), "  **掌纹 甲**\n\t与「乙」  "} {
		issue := reportCalibrationIssue{Quote: quote, Basis: strings.Repeat("据", 500)}
		raw, _ := json.Marshal(map[string]any{"issues": []reportCalibrationIssue{issue}})
		for _, body := range []reportBody{{Summary: "前" + quote + "后"}, {Content: "前" + quote + "后"}} {
			got, err := parseReportCalibration(string(raw), body)
			if err != nil || len(got) != 1 || got[0] != issue {
				t.Fatalf("legal continuous quote or rune boundary changed: issueCount=%d err=%v", len(got), err)
			}
		}
	}
	for _, tc := range []struct {
		name, quote string
		body        reportBody
	}{
		{"markdown_removed", "掌纹甲", reportBody{Content: "**掌纹**甲"}},
		{"whitespace_collapsed", "掌纹甲", reportBody{Content: "掌纹 甲"}},
		{"summary_content_joined", "摘要尾正文首", reportBody{Summary: "摘要尾", Content: "正文首"}},
		{"traditional_simplified_replaced", "对象为空", reportBody{Content: "對象為空"}},
		{"edited_character", "团队有能力", reportBody{Content: "团队有资源"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := json.Marshal(map[string]any{"issues": []reportCalibrationIssue{{Quote: tc.quote, Basis: "只检验连续原字串，不评价传统解释。"}}})
			if _, err := parseReportCalibration(string(raw), tc.body); err == nil || !strings.Contains(err.Error(), "issue=1 reason=quote_not_literal") {
				t.Fatalf("nonliteral quote was normalized or misclassified: %v", err)
			}
		})
	}
}

func TestReportCalibrationDiagnosticDoesNotReclassifySchemaOrTrailingErrors(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{`{"issues":null}`, "invalid report calibration result"},
		{`{"issues":[],"private":"PRIVATE_SCHEMA"}`, "invalid report calibration result"},
		{`{"issues":[{"quote":"PRIVATE_QUOTE","basis":"PRIVATE_BASIS","extra":"PRIVATE_SCHEMA"}]}`, "invalid report calibration result"},
		{`{"issues":[]} {"private":"PRIVATE_TRAILING"}`, "invalid trailing report calibration result"},
	} {
		if _, err := parseReportCalibration(tc.raw, reportBody{}); err == nil || err.Error() != tc.want || strings.Contains(err.Error(), "PRIVATE_") {
			t.Fatalf("schema/trailing gate changed or leaked raw input: err=%v want=%q", err, tc.want)
		}
	}
}
