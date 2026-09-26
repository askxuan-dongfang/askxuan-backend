package balance

import (
	"math"
	"testing"
)

func TestMoneyRejectsAmbiguousValues(t *testing.T) {
	for _, v := range []float64{0, -1, math.NaN(), math.Inf(1), 0.001, 1.234, 1000001} {
		if _, e := Cents(v); e == nil {
			t.Fatalf("accepted %v", v)
		}
	}
	for _, v := range []string{"-1", "1.001", "1e2", " 1", "1.", "NaN", "1000001", "1&x=1"} {
		if _, e := ParseCents(v); e == nil {
			t.Fatalf("accepted %q", v)
		}
	}
	for s, want := range map[string]int64{"0.01": 1, "19.90": 1990, "100": 10000, "1.1": 110} {
		n, e := ParseCents(s)
		if e != nil || n != want {
			t.Fatal(s, n, e)
		}
	}
}
