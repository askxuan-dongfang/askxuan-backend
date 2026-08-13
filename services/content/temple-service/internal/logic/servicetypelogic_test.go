package logic

import (
	"testing"

	"github.com/askxuan/temple-service/internal/model"
)

func TestToTypeServiceType(t *testing.T) {
	got := toTypeServiceType(&model.ServiceType{Code: "S001", Name: "祈福", Category: "法事", PriceRange: "¥100-500"})
	if got.Code != "S001" || got.Name != "祈福" || got.Category != "法事" || got.PriceRange != "¥100-500" {
		t.Fatalf("unexpected service type: %#v", got)
	}
}

func TestStandardServiceCodesAreFixed(t *testing.T) {
	if len(standardServiceCodes) != 13 {
		t.Fatalf("expected exactly 13 standard services, got %d", len(standardServiceCodes))
	}
	for _, code := range []string{"S001", "S007", "S013"} {
		if !isStandardServiceCode(code) {
			t.Fatalf("expected %s to be standard", code)
		}
	}
	for _, code := range []string{"", "S014", "CUSTOM", "S001-extra"} {
		if isStandardServiceCode(code) {
			t.Fatalf("expected %q to be rejected", code)
		}
	}
}
