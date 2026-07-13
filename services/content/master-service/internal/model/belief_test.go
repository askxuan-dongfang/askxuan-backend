package model

import "testing"

func TestNormalizeBeliefCode(t *testing.T) {
	cases := map[string]string{
		NormalizeBeliefCode("", "道教", "全真派"):    "daoism",
		NormalizeBeliefCode("", "佛教", "格鲁派"):    "tibetan_buddhism",
		NormalizeBeliefCode("", "佛教", "禅宗"):     "han_buddhism",
		NormalizeBeliefCode("folk", "佛教", "禅宗"): "folk",
	}
	for got, want := range cases {
		if got != want {
			t.Fatalf("got %s want %s", got, want)
		}
	}
}
