package logic

import "testing"

func TestCanEnableTempleAccount(t *testing.T) {
	for _, status := range []string{"正常", "推荐"} {
		if !canEnableTempleAccount(status) {
			t.Fatalf("expected status %q to permit enabling", status)
		}
	}
	for _, status := range []string{"待审核", "封禁", ""} {
		if canEnableTempleAccount(status) {
			t.Fatalf("expected status %q to deny enabling", status)
		}
	}
}
