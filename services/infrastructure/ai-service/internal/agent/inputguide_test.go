package agent

import (
	"encoding/json"
	"testing"
)

func TestGuidedCatalogPreservesAdminOptionsAndRequiresConditionalInputs(t *testing.T) {
	raw := `{"custom":"preserved","fields":[{"key":"method","type":"select","required":true,"options":[{"value":"auto","label":"自动起卦"},{"value":"number","label":"数字起卦"},{"value":"time","label":"时间起卦"}]},{"key":"numbers","type":"text","required":false},{"key":"eventTime","type":"datetime","required":false}]}`
	schema := GuidedInputSchema("liuyao", raw)
	var v map[string]interface{}
	_ = json.Unmarshal([]byte(schema), &v)
	if v["custom"] != "preserved" {
		t.Fatal("lost admin metadata")
	}
	guard := NewGuard(2000, nil)
	for _, inputs := range []map[string]interface{}{{"method": "auto"}, {"method": "number", "numbers": "12 34"}, {"method": "time", "eventTime": "2026-09-10T18:30"}} {
		if _, err := guard.Validate(schema, "工作方向", inputs); err != nil {
			t.Fatalf("valid inputs rejected: %v", err)
		}
	}
	for _, inputs := range []map[string]interface{}{{"method": "number"}, {"method": "number", "numbers": "1 2 3 4"}, {"method": "time"}, {"method": "time", "eventTime": "bad"}} {
		if _, err := guard.Validate(schema, "工作方向", inputs); err == nil {
			t.Fatalf("invalid inputs accepted: %v", inputs)
		}
	}
	restricted := GuidedInputSchema("liuyao", `{"fields":[{"key":"method","options":[{"value":"number","label":"只开放数字"}]}]}`)
	var s struct{ Fields []map[string]interface{} }
	_ = json.Unmarshal([]byte(restricted), &s)
	if _, ok := s.Fields[0]["defaultValue"]; ok {
		t.Fatal("invented default outside configured options")
	}
}
