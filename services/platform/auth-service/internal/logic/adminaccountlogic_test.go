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

func TestCanEnableMasterAccount(t *testing.T) {
	if !canEnableMasterAccount("已认证", "normal", "正常") {
		t.Fatal("verified normal master at an approved temple should be enabled")
	}
	cases := []struct {
		auth, platform, temple string
	}{
		{"待审核", "normal", "正常"},
		{"已认证", "banned", "正常"},
		{"已认证", "normal", "待审核"},
	}
	for _, tc := range cases {
		if canEnableMasterAccount(tc.auth, tc.platform, tc.temple) {
			t.Fatalf("unexpected eligible state: %+v", tc)
		}
	}
}
