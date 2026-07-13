package model

import "testing"

func TestIsValidBeliefCode(t *testing.T) {
	for _, code := range []string{BeliefHanBuddhism, BeliefTibetanBuddhism, BeliefDaoism, BeliefFolk} {
		if !IsValidBeliefCode(code) {
			t.Fatalf("expected %s to be valid", code)
		}
	}
	if IsValidBeliefCode("zen") {
		t.Fatal("specific sect must not be accepted as a top-level belief")
	}
}

func TestBuildTempleWhereBeliefAndSect(t *testing.T) {
	where, args := buildTempleWhere(TempleFilter{BeliefCode: BeliefDaoism, Sect: "全真派"})
	if where != "1=1 AND belief_code = ? AND sect = ?" {
		t.Fatalf("unexpected where: %s", where)
	}
	if len(args) != 2 || args[0] != BeliefDaoism || args[1] != "全真派" {
		t.Fatalf("unexpected args: %#v", args)
	}
}
