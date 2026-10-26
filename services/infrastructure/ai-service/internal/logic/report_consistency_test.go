package logic

import (
	"github.com/askxuan/ai-service/internal/reportdoc"
	"testing"
)

func TestReportPartnerTimeBasisIsIndependent(t *testing.T) {
	facts := map[string]any{"birthplace": "杭州市", "longitude": "120.15", "timeBasis": "true_solar", "partnerBirthplace": "乌鲁木齐市", "partnerLongitude": "87.6168", "partnerTimeBasis": "true_solar", "partnerBirthDate": "1994-08-22", "partnerBirthTime": "00:30", "partnerGender": "female", "partnerCalendarType": "solar"}
	partner := reportPartnerFacts(facts)
	if partner["birthplace"] != "乌鲁木齐市" || partner["longitude"] != "87.6168" || partner["timeBasis"] != "true_solar" {
		t.Fatal(partner)
	}
	if facts["longitude"] != "120.15" {
		t.Fatal("partner overwrote self")
	}
}

func TestRejectsObservedMarriageContradictions(t *testing.T) {
	r := &Report{SkillCode: "marriage"}
	doc := reportdoc.New()
	doc.Blocks = []reportdoc.Block{{Kind: "pillars", Source: "bazi", Title: "你 · 四柱命盘"}, {Kind: "pillars", Source: "bazi", Title: "对方 · 四柱命盘"}}
	for _, text := range []string{"女方八字与紫微命盘均未取得。因此本报告不是双人合盘。", "女方命盘未计算", "仅男方的关系模式分析"} {
		if validateReportCalculations(r, reportBody{Summary: text}, doc) == nil {
			t.Fatalf("published contradiction: %s", text)
		}
	}
	for _, text := range []string{"双方八字均已排出，以下并列对照。", "女方紫微命盘未计算，双方八字已取得。", "八字不能证明关系的现实结果。", "仅男方的时柱出现此项，女方没有。"} {
		if err := validateReportCalculations(r, reportBody{Content: text}, doc); err != nil {
			t.Fatalf("false conflict: %s: %v", text, err)
		}
	}
}
