package logic

import (
	"testing"

	"github.com/askxuan/ai-service/internal/reportdoc"
)

func TestReportLiteralCitationsPreserveRetrievedText(t *testing.T) {
	doc := reportdoc.New()
	doc.Blocks = []reportdoc.Block{{Kind: "citations", Items: []reportdoc.Item{
		{Label: "资料1", Value: "開門通利物，剛健動相加。其物圓轉通利。"},
		{Label: "资料4", Value: "又云：食神生旺，\n勝似財官，是也。"},
		{Label: "资料10", Value: "乙不栽植，千株不長。亥不嫁娶，必主分張。"},
	}}}
	for _, tc := range []struct {
		name    string
		body    reportBody
		wantErr bool
	}{
		// Report 76 changed 長/張 to 长/张 without labelling a paraphrase.
		{"report76_simplified_quotes", reportBody{Content: "原文亦记“乙不栽植，千株不长”“亥不嫁娶，必主分张”[资料10]。"}, true},
		{"simplified_in_summary", reportBody{Summary: "“乙不栽植，千株不长”[资料10]"}, true},
		{"traditional_literal", reportBody{Content: "“亥不嫁娶，必主分張”[资料10]"}, false},
		{"continuous_excerpt_without_intro", reportBody{Content: "“食神生旺，勝似財官”[资料4]"}, false},
		{"whitespace_and_punctuation", reportBody{Content: "「食神 生旺；勝似財官。」 [资料4]"}, false},
		{"ordered_ellipsis", reportBody{Content: "“開門通利物……其物圓轉通利”[资料1]"}, false},
		{"ordered_ascii_ellipsis", reportBody{Content: "\"開門通利物...剛健動相加\"[资料1]"}, false},
		{"reordered_ellipsis", reportBody{Content: "“其物圓轉通利……開門通利物”[资料1]"}, true},
		{"unknown_ref_without_quote", reportBody{Content: "解释的依据见[资料99]。"}, true},
		{"unknown_ref_in_summary", reportBody{Summary: "[资料99]"}, true},
		{"wrong_existing_source", reportBody{Content: "“開門通利物”[资料4]"}, true},
		{"one_of_attached_sources_matches", reportBody{Content: "“開門通利物”[资料4][资料1]"}, false},
		{"cannot_join_different_sources", reportBody{Content: "“開門通利物……勝似財官”[资料1][资料4]"}, true},
		{"unquoted_paraphrase", reportBody{Content: "释义：开门可取通利之象[资料1]。"}, false},
		{"ordinary_quote_without_citation", reportBody{Content: "把行动写成“记录并复盘”。"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateReportCitations(tc.body, doc)
			if (err != nil) != tc.wantErr {
				t.Fatalf("literal citation error=%v, wantErr=%t", err, tc.wantErr)
			}
		})
	}
}

func TestEarlierQuoteCannotHideBehindValidLastQuote(t *testing.T) {
	d := reportdoc.New()
	d.Blocks = []reportdoc.Block{{Kind: "citations", Items: []reportdoc.Item{{Label: "资料10", Value: "乙不栽植，千株不長。亥不嫁娶，必主分張。"}}}}
	if validateReportCitations(reportBody{Content: "“乙不栽植，千株不长”“亥不嫁娶，必主分張”[资料10]"}, d) == nil {
		t.Fatal("unchecked earlier quote passed")
	}
	if err := validateReportCitations(reportBody{Content: "“乙不栽植，千株不長”“亥不嫁娶，必主分張”[资料10]"}, d); err != nil {
		t.Fatal(err)
	}
}
