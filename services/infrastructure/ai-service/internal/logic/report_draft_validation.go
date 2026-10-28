package logic

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/askxuan/ai-service/internal/reportdoc"
)

var errReportDraftIncomplete = errors.New("incomplete report")

// This diagnostic contains counts and fixed categories only, never a parser's
// error text: JSON type errors may contain model-controlled field names.
type reportDraftDiagnostic struct {
	Category     string `json:"category"`
	RawBytes     int    `json:"rawBytes"`
	SummaryRunes int    `json:"summaryRunes"`
	ContentRunes int    `json:"contentRunes"`
	Headings     int    `json:"headings"`
	Finish       string `json:"finish"`
}

func reportDraftFailure(raw string, body reportBody, err error, finish string) reportDraftDiagnostic {
	d := reportDraftDiagnostic{Category: "json_invalid", RawBytes: len(raw), SummaryRunes: utf8.RuneCountInString(body.Summary), ContentRunes: utf8.RuneCountInString(body.Content), Headings: strings.Count(body.Content, "## "), Finish: "unknown"}
	switch finish {
	case "stop", "length", "content_filter", "tool_calls", "function_call":
		d.Finish = finish
	}
	var syntax *json.SyntaxError
	var valueType *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntax):
		d.Category = "json_syntax"
	case errors.As(err, &valueType):
		d.Category = "json_type"
	case errors.Is(err, errReportDraftIncomplete):
		switch {
		case utf8.RuneCountInString(strings.TrimSpace(body.Summary)) < 20:
			d.Category = "summary_short"
		case d.SummaryRunes > 500:
			d.Category = "summary_long"
		case utf8.RuneCountInString(strings.TrimSpace(body.Content)) < 300:
			d.Category = "content_short"
		default:
			d.Category = "headings_short"
		}
	}
	return d
}

func reportDraftInvalidLog(id int64, attempt int, d reportDraftDiagnostic) string {
	return fmt.Sprintf("report draft invalid report_id=%d attempt=%d category=%s raw_bytes=%d summary_runes=%d content_runes=%d headings=%d finish=%s", id, attempt, d.Category, d.RawBytes, d.SummaryRunes, d.ContentRunes, d.Headings, d.Finish)
}

func recordReportInvalidDraft(doc *reportdoc.Document, d reportDraftDiagnostic, reviews, protocolRepairs int, repaired bool) {
	status := "needs_repair"
	if repaired {
		status = "failed"
	}
	raw, _ := json.Marshal(map[string]any{"version": 2, "status": status, "reason": "draft_invalid", "reviews": reviews, "protocolRepairs": protocolRepairs, "issues": nil, "repaired": repaired, "diagnostic": d})
	doc.Evidence = append(doc.Evidence, reportdoc.Evidence{Tool: "report_calibration", Text: string(raw)})
}
