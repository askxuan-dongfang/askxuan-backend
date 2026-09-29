package agent

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestReviewedCatalogArguments(t *testing.T) {
	raw, e := os.ReadFile("testdata/tool_inputs.json")
	if e != nil {
		t.Fatal(e)
	}
	var fixtures map[string]map[string]any
	if e = json.Unmarshal(raw, &fixtures); e != nil {
		t.Fatal(e)
	}
	if len(ReviewedTools()) != 15 || len(fixtures) != 15 {
		t.Fatal("catalog must cover all 15 reviewed MCP tools")
	}
	export := map[string]json.RawMessage{}
	for _, def := range ReviewedTools() {
		t.Run(def.Code, func(t *testing.T) {
			fields, _ := json.Marshal(def.InputSchema)
			in, ok := fixtures[def.Code]
			if !ok {
				t.Fatal("missing fixture")
			}
			validated, e := NewGuard(20000, nil).Validate(string(fields), "合成资料联调", in)
			if e != nil {
				t.Fatal(e)
			}
			args, e := BuildToolArguments(def.Code, "合成资料联调", validated, time.Date(2026, 9, 29, 4, 30, 0, 0, time.UTC))
			if e != nil || args == "" {
				t.Fatalf("%q %v", args, e)
			}
			export[def.Code] = json.RawMessage(args)
			for _, f := range def.InputSchema.Fields {
				if f.Required {
					copy := map[string]any{}
					for k, v := range in {
						copy[k] = v
					}
					delete(copy, f.Key)
					if _, e := NewGuard(20000, nil).Validate(string(fields), "测试", copy); e == nil {
						t.Errorf("accepted missing %s", f.Key)
					}
				}
			}
		})
	}
	if dest := os.Getenv("TAIBU_FIXTURE_EXPORT"); dest != "" {
		b, _ := json.MarshalIndent(export, "", "  ")
		if e := os.WriteFile(dest, b, 0600); e != nil {
			t.Fatal(e)
		}
	}
}
func TestToolSpecificValidation(t *testing.T) {
	for _, tc := range []struct{ code, input string }{
		{"astrology", `{"astroBirthDate":"1990-01-02","astroBirthTime":"08:30","latitude":"91","longitude":"121","transitDateTime":"2026-09-29T12:30"}`},
		{"xiaoliuren", `{"lunarMonth":"8","lunarDay":"31","hourIndex":"7"}`},
		{"xiaoliuren", `{"lunarMonth":"8.5","lunarDay":"19","hourIndex":"7"}`},
		{"bazi_pillars_resolve", `{"yearPillar":"甲丑","monthPillar":"丙子","dayPillar":"丁卯","hourPillar":"甲辰"}`},
		{"meihua", `{"meihuaMethod":"number_pair","pairNumbers":"1 2 3","eventTime":"2026-09-29T12:30"}`},
	} {
		if _, e := BuildToolArguments(tc.code, "测试", tc.input, time.Now()); e == nil {
			t.Errorf("accepted %s %s", tc.code, tc.input)
		}
	}
}
