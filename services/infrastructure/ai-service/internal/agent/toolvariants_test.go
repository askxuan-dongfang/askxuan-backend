package agent

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestReviewedToolVariants(t *testing.T) {
	raw, _ := os.ReadFile("testdata/tool_inputs.json")
	var fixtures map[string]map[string]any
	_ = json.Unmarshal(raw, &fixtures)
	type variant struct {
		Tool string          `json:"tool"`
		Name string          `json:"name"`
		Args json.RawMessage `json:"args"`
	}
	results := []variant{}
	check := func(code, label string, changes map[string]any) {
		t.Run(code+"/"+label, func(t *testing.T) {
			in := map[string]any{}
			for k, v := range fixtures[code] {
				in[k] = v
			}
			for k, v := range changes {
				if v == nil {
					delete(in, k)
				} else {
					in[k] = v
				}
			}
			var def ToolDefinition
			for _, d := range ReviewedTools() {
				if d.Code == code {
					def = d
				}
			}
			fields, _ := json.Marshal(def.InputSchema)
			v, e := NewGuard(20000, nil).Validate(string(fields), "合成资料联调", in)
			if e != nil {
				t.Fatal(e)
			}
			args, e := BuildToolArguments(code, "合成资料联调", v, time.Date(2026, 9, 29, 4, 30, 0, 0, time.UTC))
			if e != nil {
				t.Fatal(e)
			}
			results = append(results, variant{code, label, json.RawMessage(args)})
		})
	}
	for _, spread := range []string{"single", "three", "love", "decision", "celtic-cross", "horseshoe", "mind-body-spirit", "situation", "yes-no"} {
		check("tarot", spread, map[string]any{"spread": spread})
	}
	for _, kind := range []string{"mutagedPlaces", "selfMutaged", "surroundedPalaces", "fliesTo"} {
		v := map[string]any{"queryType": kind}
		if kind == "fliesTo" {
			v["toPalace"] = "夫妻"
		}
		check("ziwei_flying_star", kind, v)
	}
	for _, mode := range []string{"year", "month", "day", "hour", "minute"} {
		check("taiyi", mode, map[string]any{"taiyiMode": mode})
	}
	for _, method := range []string{"auto", "time", "number", "select"} {
		v := map[string]any{"method": method, "numbers": nil}
		if method == "number" {
			v["numbers"] = "12 34 56"
		}
		if method == "time" {
			v["eventTime"] = "2026-09-29T12:30"
		}
		if method == "select" {
			v["hexagramName"] = "乾为天"
		}
		check("liuyao", method, v)
	}
	for method, v := range map[string]map[string]any{
		"time": {}, "number_pair": {"pairNumbers": "12 34"}, "number_triplet": {"tripleNumbers": "12 34 56"}, "text_split": {"divinationText": "山水"}, "count_with_time": {"count": "3", "countCategory": "item"}, "measure": {"measureKind": "丈尺", "majorValue": "1", "minorValue": "2"}, "classifier_pair": {"upperCue": "天", "lowerCue": "地"}, "select": {"hexagramName": "乾为天", "movingLine": "3"},
	} {
		v["meihuaMethod"] = method
		check("meihua", method, v)
	}
	if p := os.Getenv("TAIBU_VARIANT_EXPORT"); p != "" {
		b, _ := json.Marshal(results)
		if e := os.WriteFile(p, b, 0600); e != nil {
			t.Fatal(e)
		}
	}
}
