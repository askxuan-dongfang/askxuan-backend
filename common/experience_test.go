package common

import "testing"

func TestExperienceOrderNamespace(t *testing.T) {
	for _, tc := range []struct {
		no   string
		want bool
	}{{"EXO-abc", true}, {"SO-EXO-abc", false}, {"SO123", false}, {"exo-abc", false}, {"", false}} {
		if IsExperienceOrder(tc.no) != tc.want {
			t.Fatal(tc)
		}
	}
}
