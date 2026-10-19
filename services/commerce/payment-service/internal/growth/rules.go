// Package growth owns the independent growth ledger and platform-funded benefits.
// All monetary inputs are integer cents; clients never supply eligibility or levels.
package growth

import (
	"fmt"
	"sort"
	"time"
)

const RuleVersion = "growth-v1"
const MonthlyGrowthLimit int64 = 200
const MonthlyDiscountLimit int64 = 30000
const ObservationPeriod = 7 * 24 * time.Hour
const MaxAmount int64 = 100000000

var Shanghai = time.FixedZone("Asia/Shanghai", 8*60*60)

type Level struct {
	Code          int    `json:"code"`
	Name          string `json:"name"`
	Threshold     int64  `json:"threshold"`
	GoodsRate     int64  `json:"goodsRateBps"`
	ServiceRate   int64  `json:"serviceRateBps"`
	CouponMinimum int64  `json:"couponMinimumCents"`
	CouponValue   int64  `json:"couponValueCents"`
	Benefit       string `json:"benefit"`
}

func Levels() []Level {
	return []Level{
		{1, "初识", 0, 10000, 10000, 0, 0, "成长档案"},
		{2, "同行", 200, 9800, 10000, 10000, 500, "等级徽章"},
		{3, "笃行", 1000, 9700, 9900, 20000, 1000, "月度成长回顾"},
		{4, "致远", 3000, 9500, 9800, 30000, 2000, "平台客服优先分流"},
		{5, "恒久", 8000, 9200, 9500, 50000, 5000, "年度数字成长纪念卡"},
	}
}
func Tier(total int64) Level {
	all := Levels()
	for i := len(all) - 1; i >= 0; i-- {
		if total >= all[i].Threshold {
			return all[i]
		}
	}
	return all[0]
}
func Earned(cents int64) int64 {
	if cents < 0 {
		return 0
	}
	return cents / 100
}
func BusinessDay(t time.Time) string   { return t.In(Shanghai).Format("2006-01-02") }
func BusinessMonth(t time.Time) string { return t.In(Shanghai).Format("2006-01") }

type Line struct {
	ID             string `json:"id"`
	ItemType       string `json:"itemType"`
	ItemID         string `json:"itemId"`
	Name           string `json:"name"`
	Category       string `json:"category"` // goods, service, making, excluded
	OwnerType      string `json:"ownerType"`
	OwnerID        string `json:"ownerId"`
	Amount         int64  `json:"amountCents"`
	Quantity       int64  `json:"quantity"`
	MaxDiscount    int64  `json:"maxDiscountCents"`
	ActivityAmount int64  `json:"activityAmountCents"`
	Eligible       bool   `json:"eligible"`
	GrowthEligible bool   `json:"growthEligible"`
	Reason         string `json:"reason"`
	Discount       int64  `json:"discountCents"`
	Net            int64  `json:"netCents"`
	PolicyRevision int64  `json:"policyRevision"`
}
type Coupon struct {
	ID      int64  `db:"id" json:"id"`
	Level   int    `db:"level" json:"level"`
	Minimum int64  `db:"minimum_cents" json:"minimumCents"`
	Value   int64  `db:"value_cents" json:"valueCents"`
	Status  string `db:"status" json:"status"`
	Expires int64  `db:"expires_at" json:"expiresAt"`
}
type Offer struct {
	Kind     string `json:"kind"`
	CouponID int64  `json:"couponId"`
	Discount int64  `json:"discountCents"`
	Payable  int64  `json:"payableCents"`
	Growth   int64  `json:"estimatedGrowth"`
	Capped   bool   `json:"capped"`
	Lines    []Line `json:"lines"`
}

// Allocate uses the largest remainder method with stable line IDs for ties.
func Allocate(total int64, weights []int64, ids []string) ([]int64, error) {
	out := make([]int64, len(weights))
	if len(ids) != len(weights) || total < 0 || total > MaxAmount {
		return nil, fmt.Errorf("invalid allocation")
	}
	sum := int64(0)
	for _, w := range weights {
		if w < 0 || w > MaxAmount {
			return nil, fmt.Errorf("invalid weight")
		}
		sum += w
	}
	if sum > MaxAmount || total > sum {
		return nil, fmt.Errorf("allocation exceeds base")
	}
	if sum == 0 {
		return out, nil
	}
	type part struct {
		index     int
		remainder int64
	}
	parts := make([]part, len(weights))
	used := int64(0)
	for i, w := range weights {
		out[i] = total * w / sum
		used += out[i]
		parts[i] = part{i, total * w % sum}
	}
	sort.SliceStable(parts, func(i, j int) bool {
		if parts[i].remainder != parts[j].remainder {
			return parts[i].remainder > parts[j].remainder
		}
		return ids[parts[i].index] < ids[parts[j].index]
	})
	for i := int64(0); i < total-used; i++ {
		out[parts[i].index]++
	}
	return out, nil
}
func Candidates(lines []Line, level Level, remaining int64, coupons []Coupon) ([]Offer, error) {
	return candidatesWithRules(lines, level, remaining, coupons, DefaultRules())
}
func candidatesWithRules(lines []Line, level Level, remaining int64, coupons []Coupon, rules RuleConfig) ([]Offer, error) {
	if remaining < 0 {
		remaining = 0
	}
	if remaining > rules.MonthlyDiscountLimit {
		remaining = rules.MonthlyDiscountLimit
	}
	total := int64(0)
	ids := make([]string, len(lines))
	seen := map[string]bool{}
	for i, l := range lines {
		if l.ID == "" || seen[l.ID] || l.Amount < 0 || l.Amount > MaxAmount || l.ActivityAmount < 0 || l.ActivityAmount > l.Amount {
			return nil, fmt.Errorf("invalid line")
		}
		seen[l.ID] = true
		ids[i] = l.ID
		total += l.Amount
	}
	if total < 1 || total > MaxAmount {
		return nil, fmt.Errorf("invalid total")
	}
	build := func(kind string, couponID int64, discounts []int64, capped bool) Offer {
		o := Offer{Kind: kind, CouponID: couponID, Payable: total, Capped: capped, Lines: append([]Line{}, lines...)}
		base := int64(0)
		for i, d := range discounts {
			o.Lines[i].Discount = d
			o.Lines[i].Net = lines[i].Amount - d
			o.Discount += d
			if lines[i].GrowthEligible {
				base += o.Lines[i].Net
			}
		}
		o.Payable -= o.Discount
		o.Growth = Earned(base)
		return o
	}
	result := []Offer{build("normal", 0, make([]int64, len(lines)), false)}
	activity := make([]int64, len(lines))
	hasActivity := false
	for i, l := range lines {
		if l.ActivityAmount > 0 && l.ActivityAmount < l.Amount {
			activity[i] = l.Amount - l.ActivityAmount
			hasActivity = true
		}
	}
	if hasActivity {
		result = append(result, build("activity", 0, activity, false))
	}
	discounts := make([]int64, len(lines))
	capped := false
	for _, category := range []string{"goods", "service"} {
		weights := make([]int64, len(lines))
		sum := int64(0)
		cap := rules.GoodsCap
		rate := level.GoodsRate
		if category == "service" {
			cap = rules.ServiceCap
			rate = level.ServiceRate
		}
		if rate < 0 || rate > 10000 {
			return nil, fmt.Errorf("invalid rate")
		}
		for i, l := range lines {
			if l.Eligible && l.Category == category {
				weights[i] = l.Amount * (10000 - rate) / 10000
				sum += weights[i]
			}
		}
		n := sum
		if n > cap {
			n = cap
			capped = true
		}
		parts, e := Allocate(n, weights, ids)
		if e != nil {
			return nil, e
		}
		for i, d := range parts {
			discounts[i] += d
		}
	}
	sum := int64(0)
	for _, d := range discounts {
		sum += d
	}
	if sum > remaining {
		var e error
		discounts, e = Allocate(remaining, discounts, ids)
		if e != nil {
			return nil, e
		}
		capped = true
	}
	result = append(result, build("level", 0, discounts, capped))
	for _, c := range coupons {
		if c.Status != "unused" || c.Value < 1 || c.Value > MaxAmount || c.Minimum < 0 {
			continue
		}
		weights := make([]int64, len(lines))
		base := int64(0)
		for i, l := range lines {
			if l.Eligible && l.Category == "goods" {
				weights[i] = l.Amount
				base += l.Amount
			}
		}
		if base < c.Minimum || base == 0 {
			continue
		}
		value := c.Value
		if value >= base {
			value = base - 1
		}
		if value <= 0 {
			continue
		}
		parts, e := Allocate(value, weights, ids)
		if e != nil {
			return nil, e
		}
		result = append(result, build("coupon", c.ID, parts, false))
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Payable != result[j].Payable {
			return result[i].Payable < result[j].Payable
		}
		if (result[i].CouponID == 0) != (result[j].CouponID == 0) {
			return result[i].CouponID == 0
		}
		return result[i].CouponID < result[j].CouponID
	})
	return result, nil
}
