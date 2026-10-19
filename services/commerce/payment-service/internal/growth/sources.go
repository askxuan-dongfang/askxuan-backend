package growth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"strconv"
	"strings"
)

type reader interface {
	QueryRowPartialCtx(context.Context, any, string, ...any) error
	QueryRowsPartialCtx(context.Context, any, string, ...any) error
	QueryRowCtx(context.Context, any, string, ...any) error
	QueryRowsCtx(context.Context, any, string, ...any) error
}
type Policy struct {
	Type      string `db:"item_type" json:"itemType"`
	ID        string `db:"item_id" json:"itemId"`
	Title     string `db:"title" json:"title"`
	OwnerType string `db:"owner_type" json:"ownerType"`
	OwnerID   string `db:"owner_id" json:"ownerId"`
	Enabled   bool   `db:"enabled" json:"enabled"`
	Consent   bool   `db:"consent" json:"consent"`
	Cost      int64  `db:"cost_cents" json:"costCents"`
	Minimum   int64  `db:"minimum_contribution_cents" json:"minimumContributionCents"`
	Cap       int64  `db:"subsidy_cap_cents" json:"subsidyCapCents"`
	Revision  int64  `db:"revision" json:"revision"`
}

const policyFields = "item_type,item_id,title,owner_type,owner_id,enabled,consent,cost_cents,minimum_contribution_cents,subsidy_cap_cents,revision"

// SnapshotMoney parses only exact decimal yuan. Existing immutable booking
// snapshots distinguish the temple's fixed fee from voluntary merit_money.
func SnapshotMoney(raw json.RawMessage) (int64, error) {
	v := strings.TrimSpace(string(raw))
	if v == "" || v == "null" {
		return 0, nil
	}
	if !json.Valid(raw) || strings.HasPrefix(v, "-") {
		return 0, ErrChanged
	}
	parts := strings.Split(v, ".")
	if len(parts) > 2 {
		return 0, ErrChanged
	}
	whole, e := strconv.ParseInt(parts[0], 10, 64)
	if e != nil || whole < 0 || whole > MaxAmount/100 {
		return 0, ErrChanged
	}
	frac := int64(0)
	if len(parts) == 2 {
		if len(parts[1]) == 0 || len(parts[1]) > 2 {
			return 0, ErrChanged
		}
		f := parts[1] + strings.Repeat("0", 2-len(parts[1]))
		frac, e = strconv.ParseInt(f, 10, 64)
		if e != nil {
			return 0, ErrChanged
		}
	}
	if whole*100+frac > MaxAmount {
		return 0, ErrChanged
	}
	return whole*100 + frac, nil
}
func sourceLines(ctx context.Context, q reader, kind, no string, gross int64) ([]Line, int64, error) {
	lines := []Line{}
	expiry := int64(0)
	add := func(id, itemType, itemID, name, category, ownerType, ownerID string, cents int64) {
		if cents > 0 {
			lines = append(lines, Line{ID: id, ItemType: itemType, ItemID: itemID, Name: name, Category: category, OwnerType: ownerType, OwnerID: ownerID, Amount: cents, Quantity: 1, GrowthEligible: category != "excluded"})
		}
	}
	switch kind {
	case "shop_order", "diy_order":
		type item struct {
			ID       int64  `db:"id"`
			SKU      int64  `db:"sku_id"`
			Product  int64  `db:"product_id"`
			Name     string `db:"name"`
			Quantity int64  `db:"quantity"`
			Cents    int64  `db:"cents"`
		}
		items := []item{}
		query := "SELECT i.id,i.sku_id,i.product_id,i.product_name name,i.quantity,CAST(ROUND(i.price*i.quantity*100) AS SIGNED) cents FROM askxuan_order.shop_order_item i JOIN askxuan_order.shop_order o ON o.id=i.order_id WHERE o.order_no=? ORDER BY i.id"
		if kind == "diy_order" {
			query = "SELECT i.id,i.sku_id,i.material_id product_id,i.material_name name,i.quantity,CAST(ROUND(i.unit_price*i.quantity*100) AS SIGNED) cents FROM askxuan_diy.diy_order_item i JOIN askxuan_diy.diy_order o ON o.id=i.order_id WHERE o.order_no=? ORDER BY i.id"
		}
		if e := q.QueryRowsCtx(ctx, &items, query, no); e != nil {
			return nil, 0, e
		}
		for _, i := range items {
			itemType := "shop_sku"
			id := fmt.Sprint(i.SKU)
			if kind == "diy_order" {
				itemType = "material_sku"
				if i.SKU == 0 {
					itemType = "material"
					id = fmt.Sprint(i.Product)
				}
			}
			if i.Quantity < 1 || i.Cents < 0 {
				return nil, 0, ErrChanged
			}
			add(fmt.Sprint(i.ID), itemType, id, i.Name, "goods", "platform", "", i.Cents)
			if i.Cents > 0 {
				lines[len(lines)-1].Quantity = i.Quantity
			}
		}
		if kind == "diy_order" {
			var o struct {
				Bless    int64  `db:"bless"`
				Material int64  `db:"material"`
				Snapshot string `db:"snapshot"`
			}
			if e := q.QueryRowCtx(ctx, &o, "SELECT CAST(ROUND(bless_fee*100) AS SIGNED) bless,CAST(ROUND(material_fee*100) AS SIGNED) material,COALESCE(pricing_snapshot,'{}') snapshot FROM askxuan_diy.diy_order WHERE order_no=?", no); e != nil {
				return nil, 0, e
			}
			sum := int64(0)
			for _, l := range lines {
				sum += l.Amount
			}
			if sum != o.Material {
				return nil, 0, ErrChanged
			}
			if o.Bless > 0 {
				var snap struct {
					Consecration *struct {
						ProviderType string `json:"providerType"`
						Temple       string `json:"templeCode"`
						Master       string `json:"masterCode"`
						Service      string `json:"sourceServiceCode"`
					} `json:"consecration"`
				}
				if e := json.Unmarshal([]byte(o.Snapshot), &snap); e != nil {
					return nil, 0, e
				}
				c := snap.Consecration
				if c != nil && (c.ProviderType == "temple" || c.ProviderType == "master") {
					owner := c.Temple
					if c.ProviderType == "master" {
						owner = c.Master
					}
					add("service", c.ProviderType+"_service", owner+":"+c.Service, "附加服务", "service", c.ProviderType, owner, o.Bless)
				} else {
					add("legacy-service", "legacy", "", "附加服务", "excluded", "platform", "", o.Bless)
				}
			}
		}
	case "booking":
		var o struct {
			Master   string `db:"master_code"`
			Temple   string `db:"temple_code"`
			Code     string `db:"service_code"`
			Name     string `db:"service_name"`
			Snapshot string `db:"snapshot"`
			Expires  int64  `db:"expires"`
		}
		if e := q.QueryRowCtx(ctx, &o, "SELECT master_code,temple_code,service_code,service_name,COALESCE(CAST(price_snapshot AS CHAR),'{}') snapshot,COALESCE(UNIX_TIMESTAMP(payment_expire_time),0) expires FROM askxuan_booking.booking WHERE booking_no=?", no); e != nil {
			return nil, 0, e
		}
		expiry = o.Expires
		snap := map[string]json.RawMessage{}
		if e := json.Unmarshal([]byte(o.Snapshot), &snap); e != nil {
			return nil, 0, e
		}
		temple, e := SnapshotMoney(snap["templeFee"])
		if e != nil {
			return nil, 0, e
		}
		master, e := SnapshotMoney(snap["masterFee"])
		if e != nil {
			return nil, 0, e
		}
		if _, ok := snap["price"]; ok {
			master, e = SnapshotMoney(snap["price"])
			if e != nil {
				return nil, 0, e
			}
		}
		add("temple", "temple_service", o.Temple+":"+o.Code, o.Name+"·寺院服务", "service", "temple", o.Temple, temple)
		add("master", "master_service", o.Master+":"+o.Code, o.Name+"·执行服务", "service", "master", o.Master, master)
	case "consultation":
		var owner string
		if e := q.QueryRowCtx(ctx, &owner, "SELECT master_code FROM askxuan_booking.consultation_order WHERE order_no=?", no); e != nil {
			return nil, 0, e
		}
		add("consultation", "consultation", owner, "咨询服务", "service", "master", owner, gross)
	case "ai_report":
		var code string
		if e := q.QueryRowCtx(ctx, &code, "SELECT skill_code FROM askxuan_ai.ai_report WHERE report_no=?", no); e != nil {
			return nil, 0, e
		}
		add("report", "ai_report", code, "报告解锁", "service", "platform", "", gross)
	default:
		return nil, 0, ErrChanged
	}
	sum := int64(0)
	for _, l := range lines {
		sum += l.Amount
	}
	if sum > gross {
		return nil, 0, ErrChanged
	}
	if sum < gross {
		add("excluded", "excluded", "", "随喜、运费或其他不参与费用", "excluded", "platform", "", gross-sum)
	}
	return lines, expiry, nil
}
func applyPolicies(ctx context.Context, q reader, lines []Line, enabled bool, level Level) ([]Line, error) {
	for i := range lines {
		l := &lines[i]
		l.Reason = "项目暂未参与等级优惠"
		l.Eligible = false
		if l.Category != "goods" && l.Category != "service" {
			l.Reason = "该费用不参与折扣"
			continue
		}
		if !enabled {
			l.Reason = "等级优惠暂未开放"
			continue
		}
		var p Policy
		e := q.QueryRowCtx(ctx, &p, "SELECT "+policyFields+" FROM growth_item_policy WHERE item_type=? AND item_id=?", l.ItemType, l.ItemID)
		if errors.Is(e, sqlx.ErrNotFound) {
			continue
		}
		if e != nil {
			return nil, e
		}
		l.PolicyRevision = p.Revision
		if !p.Enabled || !p.Consent || p.OwnerType != l.OwnerType || p.OwnerID != l.OwnerID {
			continue
		}
		rate := level.GoodsRate
		if l.Category == "service" {
			rate = level.ServiceRate
		}
		limit := l.Amount - (p.Cost+p.Minimum)*l.Quantity
		if p.Cap*l.Quantity < limit {
			limit = p.Cap * l.Quantity
		}
		if limit < 0 || l.Amount*(10000-rate)/10000 > limit {
			l.Reason = "该项目当前不满足优惠条件"
			continue
		}
		l.Eligible = true
		l.MaxDiscount = limit
		l.Reason = "平台承担等级优惠，不减少提供者合同应得"
	}
	return lines, nil
}
