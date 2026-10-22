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
	if where != "1=1 AND belief_code IN (?,?) AND sect = ?" {
		t.Fatalf("unexpected where: %s", where)
	}
	if len(args) != 3 || args[0] != BeliefDaoism || args[1] != "taoism" || args[2] != "全真派" {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestBuildTempleWherePublicStatuses(t *testing.T) {
	where, args := buildTempleWhere(TempleFilter{Statuses: TemplePublicStatuses()})
	if where != "1=1 AND status IN (?,?)" {
		t.Fatalf("unexpected where: %s", where)
	}
	if len(args) != 2 || args[0] != TempleStatusNormal || args[1] != TempleStatusRecommend {
		t.Fatalf("unexpected args: %#v", args)
	}
	if !IsTemplePublicStatus(TempleStatusRecommend) || IsTemplePublicStatus(TempleStatusPending) {
		t.Fatal("unexpected public status classification")
	}
}

func TestTempleCertificationDoesNotImplyDisplay(t *testing.T) {
	for _, status := range []string{TempleStatusPending, TempleStatusBanned, "unknown"} {
		if initialTempleShelf(status) != "off_shelf" {
			t.Fatalf("uncertified initial subject displayed: %s", status)
		}
	}
	for _, auth := range []string{"pending", "legacy_unknown", "certified"} {
		for _, platform := range []string{"normal", "banned"} {
			for _, shelf := range []string{"on_shelf", "off_shelf"} {
				got := IsTemplePublic(&Temple{AuthStatus: auth, PlatformStatus: platform, ShelfStatus: shelf})
				want := auth == "certified" && platform == "normal" && shelf == "on_shelf"
				if got != want {
					t.Fatalf("public classification %s/%s/%s=%v", auth, platform, shelf, got)
				}
			}
		}
	}
}
