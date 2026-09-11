package model

import "testing"

func TestIndependentMasterBeliefCodes(t *testing.T) {
	for _, tc := range []struct{ input, want string }{{"taoism", "daoism"}, {"daoism", "daoism"}, {"han_buddhism", "han_buddhism"}, {"tibetan_buddhism", "tibetan_buddhism"}, {"folk", "folk"}} {
		if got := NormalizeBeliefCode(tc.input, "", ""); got != tc.want {
			t.Errorf("%s -> %s want %s", tc.input, got, tc.want)
		}
	}
}
