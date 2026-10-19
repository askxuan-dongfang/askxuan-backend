package growth

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestTierAndRounding(t *testing.T) {
	for _, c := range []struct {
		n     int64
		level int
	}{{0, 1}, {199, 1}, {200, 2}, {999, 2}, {1000, 3}, {2999, 3}, {3000, 4}, {7999, 4}, {8000, 5}} {
		if Tier(c.n).Code != c.level {
			t.Fatal(c)
		}
	}
	if Earned(19999) != 199 || Earned(19999-99) != 199 || Earned(19999-100) != 198 {
		t.Fatal("refund rounding")
	}
	tm := time.Date(2026, 10, 31, 16, 0, 0, 0, time.UTC)
	if BusinessDay(tm) != "2026-11-01" || BusinessMonth(tm) != "2026-11" {
		t.Fatal("business time")
	}
}
func TestPriceExamplesAndCouponChoice(t *testing.T) {
	lines := []Line{{ID: "g", Amount: 20000, Category: "goods", Eligible: true, GrowthEligible: true}, {ID: "s", Amount: 10000, Category: "service", Eligible: true, GrowthEligible: true}, {ID: "ship", Amount: 1000, Category: "excluded"}, {ID: "giving", Amount: 2000, Category: "excluded"}}
	offers, e := Candidates(lines, Tier(1000), 30000, nil)
	if e != nil || offers[0].Payable != 32300 || offers[0].Growth != 293 {
		t.Fatal(offers, e)
	}
	offers, e = Candidates(lines, Tier(1000), 30000, []Coupon{{ID: 1, Minimum: 20000, Value: 1000, Status: "unused"}})
	if e != nil || offers[0].Kind != "coupon" || offers[0].Payable != 32000 || offers[0].Growth != 290 {
		t.Fatal(offers, e)
	}
	lines[0].Amount = 200000
	lines[1].Amount = 200000
	lines = lines[:3]
	lines[2].Amount = 2000
	offers, e = Candidates(lines, Tier(8000), 30000, nil)
	if e != nil || offers[0].Payable != 387000 || !offers[0].Capped {
		t.Fatal(offers, e)
	}
	offers, e = Candidates(lines, Tier(8000), 12000, nil)
	if e != nil || offers[0].Payable != 390000 || offers[0].Lines[0].Discount != 8000 || offers[0].Lines[1].Discount != 4000 {
		t.Fatal(offers, e)
	}
	offers, e = Candidates([]Line{{ID: "g", Amount: 1999, Category: "goods", Eligible: true, GrowthEligible: true}}, Tier(200), 30000, nil)
	if e != nil || offers[0].Payable != 1960 {
		t.Fatal(offers, e)
	}
}
func TestAllocationConservesCentsAndStableTie(t *testing.T) {
	a, e := Allocate(2, []int64{1, 1, 1}, []string{"c", "a", "b"})
	if e != nil || !reflect.DeepEqual(a, []int64{0, 1, 1}) {
		t.Fatal(a, e)
	}
	for total := int64(0); total < 100; total++ {
		a, e = Allocate(total, []int64{19, 31, 50}, []string{"a", "b", "c"})
		if e != nil || a[0]+a[1]+a[2] != total || a[0] > 19 || a[1] > 31 || a[2] > 50 {
			t.Fatal(a, e)
		}
	}
}
func TestExcludedAndPromotionsNeverStack(t *testing.T) {
	o, e := Candidates([]Line{{ID: "g", Amount: 10000, ActivityAmount: 8000, Category: "goods", Eligible: true, GrowthEligible: true}, {ID: "excluded", Amount: 10000, Category: "excluded"}}, Tier(8000), 30000, []Coupon{{ID: 1, Minimum: 10000, Value: 1000, Status: "unused"}})
	if e != nil || o[0].Payable != 18000 || o[0].Kind != "activity" || o[0].Growth != 80 {
		t.Fatal(o, e)
	}
	for _, x := range o {
		if x.Lines[1].Discount != 0 {
			t.Fatal("excluded discount")
		}
	}
}
func TestInvalidMoneyRejected(t *testing.T) {
	for _, lines := range [][]Line{{{ID: "a", Amount: -1}}, {{ID: "a", Amount: MaxAmount + 1}}, {{ID: "a", Amount: 2, ActivityAmount: 3}}, {{ID: "a", Amount: 1}, {ID: "a", Amount: 1}}} {
		if _, e := Candidates(lines, Tier(1000), 30000, nil); e == nil {
			t.Fatal(lines)
		}
	}
}

func TestSnapshotMoneyExact(t *testing.T) {
	for raw, want := range map[string]int64{"0": 0, "19.9": 1990, "19.99": 1999, "null": 0} {
		got, e := SnapshotMoney(json.RawMessage(raw))
		if e != nil || got != want {
			t.Fatal(raw, got, e)
		}
	}
	for _, raw := range []string{"-1", "+1", "1.", "1.001", "1e2", "\"10\"", "10000000000000.99"} {
		if _, e := SnapshotMoney(json.RawMessage(raw)); e == nil {
			t.Fatal("invalid money", raw)
		}
	}
}
