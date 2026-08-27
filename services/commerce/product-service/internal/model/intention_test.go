package model

import (
	"strings"
	"testing"
)

func TestValidateIntentCodes(t *testing.T) {
	if err := ValidateIntentCodes([]string{"peace", "diy", "custom_wish", "peace"}); err != nil {
		t.Fatalf("valid tags rejected: %v", err)
	}
	if err := ValidateIntentCodes([]string{"bad-code"}); err == nil {
		t.Fatal("invalid code format accepted")
	}
}

func TestResourceUnionUsesQualifiedTempleCatalog(t *testing.T) {
	query, args := resourceUnion("peace")
	for _, table := range []string{
		"askxuan_temple.temple_service",
		"askxuan_temple.temple",
		"askxuan_temple.temple_service_intent_tag",
		"askxuan_master.master",
		"askxuan_master.master_service_tag",
	} {
		if !strings.Contains(query, table) {
			t.Fatalf("missing qualified table %s", table)
		}
	}
	if strings.Contains(query, "askxuan_product.product") {
		t.Fatal("intention aggregation must not include products")
	}
	if len(args) != 2 || args[0] != "peace" || args[1] != "peace" {
		t.Fatalf("unexpected args: %#v", args)
	}
}
