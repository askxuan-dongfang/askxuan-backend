package common

import "testing"

func TestAffiliationBeliefsAndCodeShape(t *testing.T) {
	for _, code := range []string{"han_buddhism", "tibetan_buddhism", "theravada_buddhism", "daoism", "folk", "taoism"} {
		got, err := CanonicalBelief(code)
		if err != nil || got == "taoism" || BeliefDisplayName(got) == "" {
			t.Fatalf("%s: %s %v", code, got, err)
		}
	}
	for _, code := range []string{"", "buddhism", "southern_buddhism", "unknown"} {
		if _, err := CanonicalBelief(code); err == nil {
			t.Fatalf("invalid belief %q", code)
		}
	}
	for _, codes := range [][]string{nil, {}, {"chan", "chan"}, {"../chan"}, make([]string, 13)} {
		if ValidateTraditionCodeShape(codes, true) == nil {
			t.Fatalf("invalid selection %#v", codes)
		}
	}
	if err := ValidateTraditionCodeShape(nil, false); err != nil {
		t.Fatal(err)
	}
	if err := ValidateTraditionCodeShape([]string{"mazu", "guandi"}, true); err != nil {
		t.Fatal(err)
	}
}
