package growth

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type CatalogItem struct {
	Type     string `json:"itemType"`
	ID       string `json:"itemId"`
	Quantity int64  `json:"quantity"`
	Spec     string `json:"spec"`
}
type CatalogPreview struct {
	Snapshot   Snapshot `json:"snapshot"`
	Gross      int64    `json:"grossCents"`
	LevelPrice *Offer   `json:"levelPrice"`
	Reason     string   `json:"reason"`
	ExpiresAt  int64    `json:"expiresAt"`
	ServerTime int64    `json:"serverTime"`
}

// Only catalog identifiers and quantities come from the client. Never accept a
// client price, owner, discount, level or contribution margin. This read-only
// preview neither creates an order nor consumes a coupon/budget reservation.
func catalogLine(ctx context.Context, q reader, user string, item CatalogItem) (Line, error) {
	l := Line{ID: item.Type + ":" + item.ID, ItemType: item.Type, ItemID: item.ID, Quantity: item.Quantity, Category: "goods", OwnerType: "platform", GrowthEligible: true}
	if item.Quantity < 1 || item.Quantity > 10000 || item.ID == "" || len(item.ID) > 96 || len(item.Spec) > 100 {
		return l, common.ErrParam
	}
	var row struct {
		Name  string `db:"name"`
		Price int64  `db:"price"`
		Stock int64  `db:"stock"`
	}
	query := ""
	args := []any{item.ID}
	switch item.Type {
	case "shop_sku":
		query = "SELECT p.name,CAST(ROUND(s.price*100) AS SIGNED) price,s.stock FROM askxuan_product.product_sku s JOIN askxuan_product.product p ON p.id=s.product_id WHERE s.id=? AND p.status='on_shelf' AND p.is_experience=0"
	case "shop_product":
		query = "SELECT name,CAST(ROUND(price*100) AS SIGNED) price,stock FROM askxuan_product.product WHERE id=? AND status='on_shelf' AND is_experience=0 AND NOT EXISTS(SELECT 1 FROM askxuan_product.product_sku WHERE product_id=product.id)"
	case "material_sku":
		query = "SELECT m.name,CAST(ROUND(s.price*100) AS SIGNED) price,LEAST(m.stock,s.stock) stock FROM askxuan_diy.material_sku s JOIN askxuan_diy.material m ON m.id=s.material_id WHERE s.id=? AND m.status='on_shelf'"
	case "material":
		var resolved struct {
			Name  string `db:"name"`
			Price int64  `db:"price"`
			Stock int64  `db:"stock"`
			SKU   int64  `db:"sku_id"`
		}
		e := q.QueryRowCtx(ctx, &resolved, "SELECT m.name,CAST(ROUND(COALESCE(s.price,m.unit_price)*100) AS SIGNED) price,LEAST(m.stock,COALESCE(s.stock,m.stock)) stock,COALESCE(s.id,0) sku_id FROM askxuan_diy.material m LEFT JOIN askxuan_diy.material_sku s ON s.material_id=m.id AND s.spec=IF(?='',m.spec,?) WHERE m.id=? AND m.status='on_shelf' AND (s.id IS NOT NULL OR m.spec=IF(?='',m.spec,?))", item.Spec, item.Spec, item.ID, item.Spec, item.Spec)
		if e != nil {
			if errors.Is(e, sqlx.ErrNotFound) {
				return l, common.NewBizError(40943, "该项目暂不可报价，请重新选择")
			}
			return l, e
		}
		if resolved.Price < 0 || resolved.Price > MaxAmount/item.Quantity || resolved.Stock < item.Quantity {
			return l, common.NewBizError(40943, "该项目暂不可报价，请重新选择")
		}
		if resolved.SKU > 0 {
			l.ItemType = "material_sku"
			l.ItemID = strconv.FormatInt(resolved.SKU, 10)
			l.ID = l.ItemType + ":" + l.ItemID
		}
		l.Name = resolved.Name
		l.Amount = resolved.Price * item.Quantity
		return l, nil
	case "temple_service", "master_service":
		parts := strings.Split(item.ID, ":")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" || item.Quantity != 1 {
			return l, common.ErrParam
		}
		args = []any{parts[0], parts[1]}
		l.Category = "service"
		l.OwnerID = parts[0]
		if item.Type == "temple_service" {
			l.OwnerType = "temple"
			query = "SELECT s.service_name name,CAST(ROUND(s.price*100) AS SIGNED) price,1 stock FROM askxuan_temple.temple_service s JOIN askxuan_temple.temple t ON t.code=s.temple_code WHERE s.temple_code=? AND s.service_code=? AND s.status='on_shelf' AND t.shelf_status='on_shelf' AND t.platform_status='normal'"
		} else {
			l.OwnerType = "master"
			query = "SELECT s.service_code name,CAST(ROUND(s.price*100) AS SIGNED) price,1 stock FROM askxuan_master.master_service_tag s JOIN askxuan_master.master m ON m.code=s.master_code WHERE s.master_code=? AND s.service_code=? AND s.status='enabled' AND m.shelf_status='on_shelf' AND m.platform_status='normal'"
		}
	case "consultation":
		if item.Quantity != 1 {
			return l, common.ErrParam
		}
		l.Category = "service"
		l.OwnerType = "master"
		l.OwnerID = item.ID
		query = "SELECT dharma_name name,CAST(ROUND(consult_fee*100) AS SIGNED) price,1 stock FROM askxuan_master.master WHERE code=? AND consult_enabled=1 AND shelf_status='on_shelf' AND platform_status='normal'"
	case "ai_report_order":
		if item.Quantity != 1 {
			return l, common.ErrParam
		}
		var report struct {
			Name  string `db:"name"`
			Price int64  `db:"price"`
			Code  string `db:"code"`
		}
		e := q.QueryRowCtx(ctx, &report, "SELECT title name,price_cents price,skill_code code FROM askxuan_ai.ai_report WHERE report_no=? AND user_id=? AND points_paid=0", item.ID, user)
		if e != nil {
			if errors.Is(e, sqlx.ErrNotFound) {
				return l, common.NewBizError(40943, "该项目暂不可报价，请重新选择")
			}
			return l, e
		}
		if report.Price < 0 || report.Price > MaxAmount {
			return l, ErrChanged
		}
		l.Name = report.Name
		l.Amount = report.Price
		l.Category = "service"
		l.ItemType = "ai_report"
		l.ItemID = report.Code
		return l, nil
	case "ai_report":
		if item.Quantity != 1 {
			return l, common.ErrParam
		}
		l.Category = "service"
		query = "SELECT title name,price_cents price,1 stock FROM askxuan_ai.ai_report_product WHERE code=? AND enabled=1"
	default:
		return l, common.ErrParam
	}
	if e := q.QueryRowCtx(ctx, &row, query, args...); e != nil {
		if errors.Is(e, sqlx.ErrNotFound) {
			return l, common.NewBizError(40943, "该项目暂不可报价，请重新选择")
		}
		return l, e
	}
	if row.Price < 0 || row.Price > MaxAmount/item.Quantity || row.Stock < item.Quantity {
		return l, common.NewBizError(40943, "该项目暂不可报价，请重新选择")
	}
	l.Name = row.Name
	l.Amount = row.Price * item.Quantity
	return l, nil
}
func (s *Store) CatalogPrice(ctx context.Context, user string, items []CatalogItem) (v CatalogPreview, e error) {
	if len(items) < 1 || len(items) > 100 {
		return v, common.ErrParam
	}
	cfg, e := s.Settings(ctx)
	if e != nil {
		return v, e
	}
	if !cfg.Enabled {
		return v, ErrUnavailable
	}
	r, e := ruleAt(ctx, s.DB, s.now().Unix())
	if e != nil {
		return v, e
	}
	a := Account{User: user}
	e = s.DB.QueryRowCtx(ctx, &a, "SELECT user_id,total,revision,badge_visible,updated_at FROM growth_account WHERE user_id=?", user)
	if e != nil && !errors.Is(e, sqlx.ErrNotFound) {
		return v, e
	}
	lines := []Line{}
	seen := map[string]bool{}
	for _, item := range items {
		l, err := catalogLine(ctx, s.DB, user, item)
		if err != nil {
			return v, err
		}
		if seen[l.ID] {
			return v, common.ErrParam
		}
		seen[l.ID] = true
		v.Gross += l.Amount
		lines = append(lines, l)
	}
	if v.Gross > MaxAmount {
		return v, common.ErrParam
	}
	v.ServerTime = s.now().Unix()
	v.ExpiresAt = v.ServerTime + 30
	snap := Snapshot{RuleVersion: r.Version, Rules: r.Config, SettingsRevision: cfg.Revision, Level: r.Config.Tier(a.Total)}
	if v.Gross == 0 {
		normal := Offer{Kind: "normal", Lines: lines}
		snap.Offers = []Offer{normal}
		snap.Selected = normal
		v.Snapshot = snap
		v.Reason = "免费项目不产生消费成长值"
		return v, nil
	}
	v.Snapshot, e = s.offersForLines(ctx, s.DB, a, lines, snap, cfg)
	if e != nil {
		return v, e
	}
	v.Reason = "当前暂无可用等级优惠"
	for i := range v.Snapshot.Offers {
		if v.Snapshot.Offers[i].Kind == "level" {
			v.LevelPrice = &v.Snapshot.Offers[i]
			v.Reason = "等级优惠以收银台确认为准"
			break
		}
	}
	if v.LevelPrice == nil {
		for _, l := range v.Snapshot.Selected.Lines {
			if !l.Eligible {
				v.Reason = l.Reason
				break
			}
		}
	}
	return v, nil
}
