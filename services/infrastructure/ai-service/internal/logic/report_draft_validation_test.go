package logic

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/askxuan/ai-service/internal/reportdoc"
)

func TestReportDraftFailureUsesSafeCategoriesAndCounts(t *testing.T) {
	valid := reportBody{Summary: strings.Repeat("摘", 30), Content: "## 一\n## 二\n## 三\n" + strings.Repeat("文", 320)}
	encode := func(body reportBody) string { raw, _ := json.Marshal(body); return string(raw) }
	for _, tc := range []struct{ name, raw, category string }{
		{"syntax", "PRIVATE DRAFT: not JSON", "json_syntax"},
		{"field_type", `{"summary":{"PRIVATE FIELD":"PRIVATE VALUE"},"content":"PRIVATE BODY"}`, "json_type"},
		{"null", "null", "summary_short"},
		{"short_summary", encode(reportBody{Summary: strings.Repeat("摘", 19), Content: valid.Content}), "summary_short"},
		{"long_summary", encode(reportBody{Summary: strings.Repeat("摘", 501), Content: valid.Content}), "summary_long"},
		{"short_content", encode(reportBody{Summary: valid.Summary, Content: "## 一\n## 二\n## 三\n短"}), "content_short"},
		{"headings", encode(reportBody{Summary: valid.Summary, Content: strings.Repeat("文", 320)}), "headings_short"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := parseReportBody(tc.raw)
			if err == nil {
				t.Fatal("invalid report accepted")
			}
			diagnostic := reportDraftFailure(tc.raw, body, err, "PRIVATE FINISH\nstop")
			if diagnostic.Category != tc.category || diagnostic.RawBytes != len(tc.raw) || diagnostic.Finish != "unknown" {
				t.Fatal("lost safe parser classification/counts or invented finish")
			}
			log := reportDraftInvalidLog(92, 1, diagnostic)
			want := fmt.Sprintf("report draft invalid report_id=92 attempt=1 category=%s raw_bytes=%d summary_runes=%d content_runes=%d headings=%d finish=unknown", tc.category, len(tc.raw), len([]rune(body.Summary)), len([]rune(body.Content)), strings.Count(body.Content, "## "))
			if log != want || strings.Contains(log, "PRIVATE") {
				t.Fatal("unsafe format log or incorrect counts")
			}
			doc := reportdoc.New()
			recordReportInvalidDraft(&doc, diagnostic, 1, 1, false)
			recordReportInvalidDraft(&doc, diagnostic, 1, 1, true)
			audit := reportFailureAudit(doc)
			if len(audit.Evidence) != 2 {
				t.Fatal("safe format diagnosis was lost from failure audit")
			}
			for index, evidence := range audit.Evidence {
				var value map[string]any
				if json.Unmarshal([]byte(evidence.Text), &value) != nil || len(value) != 8 || value["reason"] != "draft_invalid" || value["repaired"] != (index == 1) || value["reviews"] != float64(1) || value["protocolRepairs"] != float64(1) {
					t.Fatal("format diagnostic changed stable audit classifications or counters")
				}
				wantStatus := []string{"needs_repair", "failed"}[index]
				if value["status"] != wantStatus || len(value["diagnostic"].(map[string]any)) != 6 || strings.Contains(evidence.Text, "PRIVATE") {
					t.Fatal("format audit leaked text or loosened final failure")
				}
			}
		})
	}
	_, err := parseReportBody("not JSON")
	for _, finish := range []string{"stop", "length", "content_filter", "tool_calls", "function_call"} {
		if reportDraftFailure("not JSON", reportBody{}, err, finish).Finish != finish {
			t.Fatal("lost allowlisted finish")
		}
	}
}
