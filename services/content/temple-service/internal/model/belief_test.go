package model

import "testing"

func TestIsValidBeliefCode(t *testing.T) {
	for _, code := range []string{BeliefHanBuddhism, BeliefTibetanBuddhism, BeliefDaoism, BeliefFolk, "southern_buddhism"} {
		if !IsValidBeliefCode(code) {
			t.Fatalf("expected %s to be valid", code)
		}
	}
	for _, code := range []string{"", "UPPER", "has-dash", "x"} {
		if IsValidBeliefCode(code) {
			t.Fatalf("invalid dynamic code accepted: %q", code)
		}
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
