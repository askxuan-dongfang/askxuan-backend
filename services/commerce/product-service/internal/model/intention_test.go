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
	for _, table := range []string{"askxuan_temple.temple_service", "askxuan_temple.temple", "askxuan_temple.temple_service_intent_tag"} {
		if !strings.Contains(query, table) {
			t.Fatalf("missing qualified table %s", table)
		}
	}
	if len(args) != 2 || args[0] != "peace" || args[1] != "peace" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestUniqueCodes(t *testing.T) {
	got := uniqueCodes([]string{"peace", " peace ", "", "diy"})
	if len(got) != 2 || got[0] != "peace" || got[1] != "diy" {
		t.Fatalf("unexpected unique tags: %#v", got)
	}
}
