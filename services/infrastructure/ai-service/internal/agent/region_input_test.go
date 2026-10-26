package agent

import (
	"encoding/json"
	"testing"
)

func TestRegionFormsDoNotAskForCoordinatesAndOldDraftsStillValidate(t *testing.T) {
	schema := GuidedInputSchema("marriage", `{"fields":[{"key":"longitude","type":"number"},{"key":"partnerLongitude","type":"number"}]}`)
	var decoded InputSchema
	if err := json.Unmarshal([]byte(schema), &decoded); err != nil {
		t.Fatal(err)
	}
	for _, f := range decoded.Fields {
		if f.Key == "longitude" || f.Key == "partnerLongitude" {
			t.Fatal("new form asks for coordinates")
		}
	}
	g := NewGuard(2000, nil)
	inputs := map[string]interface{}{"birthplace": "中国浙江省杭州市西湖区", "timeBasis": "true_solar", "partnerBirthplace": "中国新疆乌鲁木齐市沙依巴克区", "partnerTimeBasis": "true_solar"}
	if _, err := g.Validate(schema, "比较双方沟通方式", inputs); err != nil {
		t.Fatal(err)
	}
	inputs["longitude"], inputs["partnerLongitude"] = "120.13", "87.60"
	if _, err := g.Validate(schema, "旧页面提交", inputs); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{"NaN", "181", "-181", "abc"} {
		inputs["partnerLongitude"] = invalid
		if _, err := g.Validate(schema, "旧页面提交", inputs); err == nil {
			t.Fatalf("accepted %s", invalid)
		}
	}
	if _, err := g.Validate(`{"fields":[]}`, "无排盘专题", map[string]interface{}{"longitude": "120"}); err == nil {
		t.Fatal("coordinate accepted outside solar schema")
	}
}
