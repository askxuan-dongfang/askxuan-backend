package model

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"reflect"

	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Publications never update their design/price snapshots. diy_design holds the
// editable draft and an explicit pointer to the currently published version.
type DesignPublication struct {
	Id                  int64   `db:"id" json:"id"`
	DesignId            int64   `db:"design_id" json:"designId"`
	DesignRevision      int64   `db:"design_revision" json:"designRevision"`
	CreatorId           string  `db:"creator_id" json:"creatorId"`
	RewardCreatorId     string  `db:"reward_creator_id" json:"-"`
	RewardPublicationId int64   `db:"reward_publication_id" json:"-"`
	OrderId             int64   `db:"order_id" json:"-"`
	CompositionHash     string  `db:"composition_hash" json:"-"`
	DesignSnapshot      string  `db:"design_snapshot" json:"-"`
	PricingSnapshot     string  `db:"pricing_snapshot" json:"-"`
	MaterialFee         float64 `db:"material_fee" json:"materialFee"`
	PublishedAt         string  `db:"published_at" json:"publishedAt"`
}

type PublicationEligibility struct {
	CanPublish     bool
	Reason         string
	Message        string
	OrderId        int64
	OrderNo        string
	DesignRevision int64
}

var ErrPublicationUnavailable = common.NewBizError(40915, "作品没有有效定稿，请在订单付款并确认款式、用料和价格后发布")

const publicationFields = "p.id,p.design_id,p.design_revision,p.creator_id,p.reward_creator_id,p.reward_publication_id,p.order_id,p.composition_hash,p.design_snapshot,p.pricing_snapshot,p.material_fee,DATE_FORMAT(p.published_at,'%Y-%m-%d %H:%i:%s') AS published_at"

// Public browsing resolves only the current published version.
const publicationJoin = ` FROM askxuan_diy.diy_design_publication p
 JOIN askxuan_diy.diy_design d ON d.id=p.design_id AND d.publication_id=p.id AND d.status='public'
 JOIN askxuan_diy.diy_order o ON o.id=p.order_id AND o.design_id=p.design_id AND o.user_id=p.creator_id `

// Attribution may refer to an older immutable version after a replacement was
// published. An explicit unpublish or an invalid confirming order still stops
// new attribution; a new public version alone does not invalidate an old style.
const attributionPublicationJoin = ` FROM askxuan_diy.diy_design_publication p
 JOIN askxuan_diy.diy_design d ON d.id=p.design_id AND d.status='public'
 JOIN askxuan_diy.diy_order o ON o.id=p.order_id AND o.design_id=p.design_id AND o.user_id=p.creator_id `

const publicationOrderValid = `o.payment_status='success'
 AND o.status IN ('in_making','awaiting_blessing','blessing_in_progress','blessing_completed','awaiting_shipment','shipped','completed')
 AND NOT EXISTS(SELECT 1 FROM askxuan_payment.refund f JOIN askxuan_payment.payment rp ON rp.id=f.payment_id WHERE rp.order_no=o.order_no AND rp.order_type='diy_order' AND f.status IN ('pending','processing','success'))`

const publicationValid = publicationOrderValid + `
 AND EXISTS(SELECT 1 FROM askxuan_payment.payment pay WHERE pay.order_no=o.order_no AND pay.order_type='diy_order' AND pay.user_id=o.user_id AND pay.status='success' AND pay.channel IN ('wechat','alipay','balance','mock','demo_balance'))`

// Approved demos may be shown, but reward attribution still requires real payment.
const attributionPublicationValid = publicationOrderValid + `
 AND EXISTS(SELECT 1 FROM askxuan_payment.payment pay WHERE pay.order_no=o.order_no AND pay.order_type='diy_order' AND pay.user_id=o.user_id AND pay.status='success' AND pay.channel IN ('wechat','alipay','balance'))`

func FindPublication(ctx context.Context, db sqlx.Session, id int64, lock bool) (*DesignPublication, error) {
	return findPublication(ctx, db, id, lock, publicationJoin, true)
}

func FindAttributionPublication(ctx context.Context, db sqlx.Session, id int64, lock bool) (*DesignPublication, error) {
	return findPublication(ctx, db, id, lock, attributionPublicationJoin, false)
}

func findPublication(ctx context.Context, db sqlx.Session, id int64, lock bool, join string, allowDemo bool) (*DesignPublication, error) {
	if id < 1 {
		return nil, sqlx.ErrNotFound
	}
	valid := attributionPublicationValid
	if allowDemo {
		valid = publicationValid
	}
	query := "SELECT " + publicationFields + join + " WHERE p.id=? AND " + valid
	if lock {
		query += " FOR UPDATE"
	}
	var p DesignPublication
	if err := db.QueryRowCtx(ctx, &p, query, id); err != nil {
		return nil, err
	}
	if lock {
		validPayment, hasRefund, err := lockPublicationPayments(ctx, db, p.OrderId, p.CreatorId, allowDemo)
		if err != nil {
			return nil, err
		}
		if !validPayment || hasRefund {
			return nil, sqlx.ErrNotFound
		}
	}
	return &p, nil
}

// Direct balance refunds lock payment rather than the DIY order. Serialize on
// the same rows, then read current refund state under lock. Lock the full refund
// index range as well so a new pending refund cannot race the publication/order.
// Shared locks preserve the payment service's write ownership and SELECT-only grants.
func lockPublicationPayments(ctx context.Context, db sqlx.Session, orderID int64, user string, allowDemo bool) (valid, refunded bool, err error) {
	var payments []struct {
		Id      int64  `db:"id"`
		User    string `db:"user_id"`
		Channel string `db:"channel"`
		Status  string `db:"status"`
	}
	err = db.QueryRowsCtx(ctx, &payments, `SELECT id,user_id,channel,status FROM askxuan_payment.payment
 WHERE order_type='diy_order' AND order_no=(SELECT order_no FROM askxuan_diy.diy_order WHERE id=?)
 ORDER BY id FOR SHARE`, orderID)
	if err != nil {
		return false, false, err
	}
	for _, payment := range payments {
		if payment.User == user && payment.Status == "success" &&
			(payment.Channel == "wechat" || payment.Channel == "alipay" || payment.Channel == "balance" ||
				(allowDemo && (payment.Channel == "mock" || payment.Channel == "demo_balance"))) {
			valid = true
		}
		var refunds []string
		if err = db.QueryRowsCtx(ctx, &refunds, "SELECT status FROM askxuan_payment.refund WHERE payment_id=? FOR SHARE", payment.Id); err != nil {
			return false, false, err
		}
		for _, status := range refunds {
			if status == "pending" || status == "processing" || status == "success" {
				refunded = true
			}
		}
	}
	return valid, refunded, nil
}

func PublicationDesign(p *DesignPublication) (*DiyDesign, error) {
	var d DiyDesign
	if p == nil || json.Unmarshal([]byte(p.DesignSnapshot), &d) != nil || d.Id != p.DesignId || d.UserId != p.CreatorId {
		return nil, ErrPublicationUnavailable
	}
	d.PublicationId, d.Status, d.TotalPrice = p.Id, DesignStatusPublic, p.MaterialFee
	return &d, nil
}

func PublishedDesign(ctx context.Context, db sqlx.Session, d *DiyDesign) (*DiyDesign, *DesignPublication, error) {
	if d == nil || d.Status != DesignStatusPublic {
		return nil, nil, sqlx.ErrNotFound
	}
	p, err := FindPublication(ctx, db, d.PublicationId, false)
	if err != nil {
		return nil, nil, err
	}
	published, err := PublicationDesign(p)
	return published, p, err
}

func findPublishedDesigns(ctx context.Context, db sqlx.Session, page, size int) ([]*DiyDesign, int64, error) {
	var total int64
	if err := db.QueryRowCtx(ctx, &total, "SELECT COUNT(*)"+publicationJoin+" WHERE "+publicationValid); err != nil {
		return nil, 0, err
	}
	publications := []*DesignPublication{}
	err := db.QueryRowsCtx(ctx, &publications, "SELECT "+publicationFields+publicationJoin+" WHERE "+publicationValid+" ORDER BY p.published_at DESC,p.id DESC LIMIT ?,?", (page-1)*size, size)
	if err != nil {
		return nil, 0, err
	}
	list := make([]*DiyDesign, 0, len(publications))
	for _, p := range publications {
		d, err := PublicationDesign(p)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, d)
	}
	return list, total, nil
}

type compositionItem struct {
	MaterialId int64  `json:"materialId"`
	Spec       string `json:"spec"`
}
type composition struct {
	Version        int               `json:"version"`
	WristSizeMm    float64           `json:"wristSizeMm"`
	FitAllowanceMm float64           `json:"fitAllowanceMm"`
	Beads          []compositionItem `json:"beads"`
	Cord           *compositionItem  `json:"cord"`
}

func readComposition(raw string) (composition, error) {
	var c composition
	if len(raw) > 128*1024 || json.Unmarshal([]byte(raw), &c) != nil ||
		c.Version != 2 || len(c.Beads) < 1 || len(c.Beads) > 60 ||
		c.WristSizeMm < 100 || c.WristSizeMm > 260 || c.FitAllowanceMm < 0 || c.FitAllowanceMm > 30 {
		return c, ErrPublicationUnavailable
	}
	for _, bead := range c.Beads {
		if bead.MaterialId < 1 || len(bead.Spec) > 100 {
			return c, ErrPublicationUnavailable
		}
	}
	if c.Cord != nil && (c.Cord.MaterialId < 1 || len(c.Cord.Spec) > 100) {
		return c, ErrPublicationUnavailable
	}
	return c, nil
}

func CompositionFingerprint(raw string) (string, error) {
	c, err := readComposition(raw)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:]), nil
}

func compositionBill(c composition) map[compositionItem]int {
	bill := make(map[compositionItem]int)
	for _, bead := range c.Beads {
		bill[bead]++
	}
	if c.Cord != nil {
		bill[*c.Cord]++
	}
	return bill
}

func matchesCompositionItems(raw string, items []PricedOrderItemInput) bool {
	c, err := readComposition(raw)
	if err != nil {
		return false
	}
	bill := make(map[compositionItem]int)
	for _, item := range items {
		if item.MaterialId < 1 || item.Quantity < 1 {
			return false
		}
		bill[compositionItem{item.MaterialId, item.Spec}] += item.Quantity
	}
	return reflect.DeepEqual(compositionBill(c), bill)
}

// Old private drafts stored only an item bill. Preserve ordering that exact bill,
// but never use it as evidence of a publishable style or reward attribution.
func matchesLegacyCheckoutItems(raw string, items []PricedOrderItemInput) bool {
	if len(raw) > 128*1024 {
		return false
	}
	var direct []DiyOrderItem
	if json.Unmarshal([]byte(raw), &direct) != nil {
		var wrapped struct {
			Version   int             `json:"version"`
			Beads     json.RawMessage `json:"beads"`
			Items     []DiyOrderItem  `json:"items"`
			Materials []DiyOrderItem  `json:"materials"`
		}
		if json.Unmarshal([]byte(raw), &wrapped) != nil || wrapped.Version > 1 || len(wrapped.Beads) > 0 {
			return false
		}
		direct = wrapped.Items
		if len(direct) == 0 {
			direct = wrapped.Materials
		}
	}
	if len(direct) == 0 {
		return false
	}
	expected, requested := make(map[compositionItem]int), make(map[compositionItem]int)
	for _, item := range direct {
		if item.MaterialId < 1 || item.Quantity < 1 {
			return false
		}
		expected[compositionItem{item.MaterialId, item.Spec}] += item.Quantity
	}
	for _, item := range items {
		if item.MaterialId < 1 || item.Quantity < 1 {
			return false
		}
		requested[compositionItem{item.MaterialId, item.Spec}] += item.Quantity
	}
	return reflect.DeepEqual(expected, requested)
}

// Confirm both immutable snapshots agree. A stale/forged bill cannot qualify a
// design merely because its designId/revision appears on a paid order.
func confirmedOrderDesign(o *DiyOrder) (*DiyDesign, string, error) {
	var d DiyDesign
	var price pricingSnapshot
	if json.Unmarshal([]byte(o.DesignSnapshot), &d) != nil || json.Unmarshal([]byte(o.PricingSnapshot), &price) != nil ||
		d.Id != o.DesignId || d.UserId != o.UserId || d.Revision < 1 || len(price.Items) == 0 ||
		o.MaterialFee <= 0 || math.IsNaN(o.MaterialFee) || math.IsInf(o.MaterialFee, 0) {
		return nil, "", ErrPublicationUnavailable
	}
	fingerprint, err := CompositionFingerprint(d.DesignData)
	if err != nil {
		return nil, "", err
	}
	inputs := make([]PricedOrderItemInput, 0, len(price.Items))
	unitPrices := make(map[compositionItem]float64)
	var fee float64
	for _, item := range price.Items {
		if item == nil || item.UnitPrice < 0 || math.IsNaN(item.UnitPrice) || math.IsInf(item.UnitPrice, 0) || item.Quantity < 1 {
			return nil, "", ErrPublicationUnavailable
		}
		key := compositionItem{item.MaterialId, item.Spec}
		if previous, found := unitPrices[key]; found && math.Abs(previous-item.UnitPrice) >= .005 {
			return nil, "", ErrPublicationUnavailable
		}
		unitPrices[key] = item.UnitPrice
		inputs = append(inputs, PricedOrderItemInput{MaterialId: item.MaterialId, Spec: item.Spec, Quantity: item.Quantity})
		fee = roundMoney(fee + item.UnitPrice*float64(item.Quantity))
	}
	if !matchesCompositionItems(d.DesignData, inputs) || math.Abs(fee-o.MaterialFee) >= .005 ||
		math.Abs(price.MaterialFee-o.MaterialFee) >= .005 || math.Abs(price.BlessFee-o.BlessFee) >= .005 ||
		math.Abs(price.TotalFee-o.TotalFee) >= .005 || math.Abs(o.MaterialFee+o.BlessFee-o.TotalFee) >= .005 {
		return nil, "", ErrPublicationUnavailable
	}
	// Keep the ordered appearance, but display prices from the same confirmed
	// order bill rather than old client/catalog values saved in design_data.
	var document map[string]json.RawMessage
	if json.Unmarshal([]byte(d.DesignData), &document) != nil {
		return nil, "", ErrPublicationUnavailable
	}
	var beads []map[string]interface{}
	if json.Unmarshal(document["beads"], &beads) != nil {
		return nil, "", ErrPublicationUnavailable
	}
	for _, bead := range beads {
		id, _ := bead["materialId"].(float64)
		spec, _ := bead["spec"].(string)
		bead["unitPrice"] = unitPrices[compositionItem{int64(id), spec}]
	}
	document["beads"], _ = json.Marshal(beads)
	if raw := document["cord"]; len(raw) > 0 && string(raw) != "null" {
		var cord map[string]interface{}
		if json.Unmarshal(raw, &cord) != nil {
			return nil, "", ErrPublicationUnavailable
		}
		id, _ := cord["materialId"].(float64)
		spec, _ := cord["spec"].(string)
		cord["unitPrice"] = unitPrices[compositionItem{int64(id), spec}]
		document["cord"], _ = json.Marshal(cord)
	}
	document["items"], _ = json.Marshal(price.Items)
	data, err := json.Marshal(document)
	if err != nil {
		return nil, "", err
	}
	d.DesignData, d.TotalPrice = string(data), o.MaterialFee
	return &d, fingerprint, nil
}

func eligibility(reason string) PublicationEligibility {
	messages := map[string]string{
		"ready":                 "订单已付款且款式、用料和价格已确认，可以发布",
		"order_required":        "完成设计并下单，付款且确认款式、用料和价格后可发布",
		"payment_required":      "订单完成付款后才可发布",
		"confirmation_required": "订单已付款，待确认款式、用料和价格后可发布",
		"design_changed":        "草稿已修改，请为当前款式下单并完成付款与定稿确认后发布新定稿",
		"order_inactive":        "定稿订单已取消、退款或处于售后中，暂不可发布",
		"snapshot_unavailable":  "订单缺少有效定稿快照，请重新下单并确认当前款式",
		"moderation_required":   "作品已被管理端下架，请修改草稿后重新确认发布资格",
	}
	return PublicationEligibility{CanPublish: reason == "ready", Reason: reason, Message: messages[reason]}
}

type publicationOrder struct {
	DiyOrder
	ValidPayment bool `db:"valid_payment"`
	HasRefund    bool `db:"has_refund"`
}

func confirmedStatus(status string) bool {
	switch status {
	case DiyStatusInMaking, DiyStatusAwaitingBlessing, DiyStatusBlessingInProgress, DiyStatusBlessingCompleted, DiyStatusAwaitingShipment, DiyStatusShipped, DiyStatusCompleted:
		return true
	default:
		return false
	}
}

func ReadPublicationEligibility(ctx context.Context, db sqlx.Session, d *DiyDesign, lock bool) (PublicationEligibility, *DiyOrder, error) {
	if d.Status == DesignStatusRejected {
		return eligibility("moderation_required"), nil, nil
	}
	fingerprint, err := CompositionFingerprint(d.DesignData)
	if err != nil {
		return eligibility("snapshot_unavailable"), nil, nil
	}
	query := `SELECT ` + diyOrderRows + `,
 EXISTS(SELECT 1 FROM askxuan_payment.payment pay WHERE pay.order_no=diy_order.order_no AND pay.order_type='diy_order' AND pay.user_id=diy_order.user_id AND pay.status='success' AND pay.channel IN ('wechat','alipay','balance','mock','demo_balance')) AS valid_payment,
 EXISTS(SELECT 1 FROM askxuan_payment.refund f JOIN askxuan_payment.payment rp ON rp.id=f.payment_id WHERE rp.order_no=diy_order.order_no AND rp.order_type='diy_order' AND f.status IN ('pending','processing','success')) AS has_refund
 FROM askxuan_diy.diy_order WHERE design_id=? AND user_id=?
 ORDER BY EXISTS(SELECT 1 FROM askxuan_payment.payment pay WHERE pay.order_no=diy_order.order_no AND pay.order_type='diy_order' AND pay.user_id=diy_order.user_id AND pay.status='success' AND pay.channel IN ('wechat','alipay','balance')) DESC,id DESC`
	if lock {
		query += " FOR UPDATE"
	}
	var orders []*publicationOrder
	if err := db.QueryRowsCtx(ctx, &orders, query, d.Id, d.UserId); err != nil {
		return PublicationEligibility{}, nil, err
	}
	result := eligibility("order_required")
	// A valid older same-composition order may qualify even if a more recent
	// attempt was cancelled or simulated. Preserve an eligible real reward source.
	priorities := map[string]int{"order_required": 0, "snapshot_unavailable": 1, "design_changed": 2, "order_inactive": 3, "payment_required": 4, "confirmation_required": 5}
	for _, order := range orders {
		snapshot, hash, snapshotErr := confirmedOrderDesign(&order.DiyOrder)
		if lock && snapshotErr == nil && hash == fingerprint {
			validPayment, hasRefund, err := lockPublicationPayments(ctx, db, order.Id, d.UserId, true)
			if err != nil {
				return PublicationEligibility{}, nil, err
			}
			order.ValidPayment, order.HasRefund = validPayment, hasRefund
		}
		reason := ""
		switch {
		case snapshotErr != nil:
			reason = "snapshot_unavailable"
		case hash != fingerprint:
			reason = "design_changed"
		case order.HasRefund || order.Status == DiyStatusCancelled || order.Status == DiyStatusInReturn || order.PaymentStatus == "refunding" || order.PaymentStatus == "refunded":
			reason = "order_inactive"
		case order.PaymentStatus != "success" || !order.ValidPayment:
			reason = "payment_required"
		case !confirmedStatus(order.Status):
			reason = "confirmation_required"
		default:
			result = eligibility("ready")
			result.OrderId, result.OrderNo, result.DesignRevision = order.Id, order.OrderNo, snapshot.Revision
			return result, &order.DiyOrder, nil
		}
		if priorities[reason] > priorities[result.Reason] {
			result = eligibility(reason)
			result.OrderId, result.OrderNo = order.Id, order.OrderNo
			if snapshot != nil {
				result.DesignRevision = snapshot.Revision
			}
		}
	}
	return result, nil, nil
}

func DraftDiffers(d *DiyDesign, p *DesignPublication) bool {
	if p == nil {
		return false
	}
	published, err := PublicationDesign(p)
	if err != nil {
		return true
	}
	currentHash, err := CompositionFingerprint(d.DesignData)
	return err != nil || currentHash != p.CompositionHash || d.Name != published.Name || d.Description != published.Description
}

// Publish and unpublish serialize with draft saves and use the existing
// optimistic revision contract. Admin publication follows the same paid gate.
func ChangePublicationStatus(ctx context.Context, db sqlx.SqlConn, id, revision int64, user, status string, admin bool) (*DiyDesign, error) {
	var result *DiyDesign
	err := db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var d DiyDesign
		if err := tx.QueryRowCtx(ctx, &d, "SELECT "+designFields+" FROM "+diyDesignTable+" WHERE id=? FOR UPDATE", id); err != nil {
			return err
		}
		if !admin && d.UserId != user {
			return common.ErrForbidden
		}
		if d.Revision != revision {
			return common.NewBizError(40914, "设计已在其他页面更新，请重新打开后编辑")
		}
		if status == DesignStatusPublic {
			eligible, order, err := ReadPublicationEligibility(ctx, tx, &d, true)
			if err != nil {
				return err
			}
			if !eligible.CanPublish || order == nil {
				return common.NewBizError(40915, eligible.Message)
			}
			snapshot, hash, err := confirmedOrderDesign(order)
			if err != nil {
				return err
			}
			// The style and prices are frozen from the approved order. The author
			// may name/describe the version without changing the approved bill.
			snapshot.Name, snapshot.Description = d.Name, d.Description
			snapshot.SourceDesignId, snapshot.SourcePublicationId = d.SourceDesignId, d.SourcePublicationId
			snapshot.Revision, snapshot.Status, snapshot.PublicationId = d.Revision+1, DesignStatusPublic, 0
			data, err := json.Marshal(snapshot)
			if err != nil {
				return err
			}
			rewardCreator, rewardPublication := d.UserId, int64(0)
			if d.SourcePublicationId > 0 {
				// Read the immutable origin even if it is now hidden. Republishing
				// an unchanged copy must never wash a revoked origin into a new
				// beneficiary. Checkout separately checks the root's validity.
				var source DesignPublication
				if err := tx.QueryRowCtx(ctx, &source, "SELECT "+publicationFields+" FROM askxuan_diy.diy_design_publication p WHERE p.id=?", d.SourcePublicationId); err != nil {
					return err
				}
				if source.CompositionHash == hash {
					rewardCreator, rewardPublication = source.RewardCreatorId, source.RewardPublicationId
					if rewardPublication == 0 {
						rewardPublication = source.Id
					}
				}
			}
			inserted, err := tx.ExecCtx(ctx, `INSERT INTO askxuan_diy.diy_design_publication(design_id,design_revision,creator_id,reward_creator_id,reward_publication_id,order_id,composition_hash,design_snapshot,pricing_snapshot,material_fee) VALUES(?,?,?,?,?,?,?,?,?,?)`, d.Id, d.Revision+1, d.UserId, rewardCreator, rewardPublication, order.Id, hash, string(data), order.PricingSnapshot, order.MaterialFee)
			if err != nil {
				return err
			}
			d.PublicationId, err = inserted.LastInsertId()
			if err != nil {
				return err
			}
		}
		if _, err := tx.ExecCtx(ctx, "UPDATE "+diyDesignTable+" SET status=?,publication_id=?,revision=revision+1,update_time=NOW() WHERE id=? AND revision=?", status, d.PublicationId, d.Id, revision); err != nil {
			return err
		}
		d.Status, d.Revision = status, revision+1
		result = &d
		return nil
	})
	return result, err
}

// Lock the draft or public snapshot before pricing. In particular a stale UI
// cannot order/attribute a different revision after another device saves it.
func validateOrderDesign(ctx context.Context, tx sqlx.Session, in *PricedOrderInput) error {
	var current DiyDesign
	if err := tx.QueryRowCtx(ctx, &current, "SELECT "+designFields+" FROM "+diyDesignTable+" WHERE id=? FOR UPDATE", in.Design.Id); err != nil {
		return err
	}
	var publication *DesignPublication
	if in.Source == "design_square" {
		if current.PublicationId != in.Design.PublicationId || current.Status != DesignStatusPublic {
			return ErrPublicationUnavailable
		}
		p, err := FindPublication(ctx, tx, current.PublicationId, true)
		if err != nil {
			return ErrPublicationUnavailable
		}
		published, err := PublicationDesign(p)
		if err != nil {
			return err
		}
		in.Design = published
		publication, err = FindAttributionPublication(ctx, tx, p.Id, true)
		if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
			return err
		}
	} else {
		if current.UserId != in.UserId || current.Revision != in.Design.Revision {
			return common.NewBizError(40914, "设计已更新，请重新打开后下单")
		}
		in.Design = &current
		if current.SourcePublicationId > 0 {
			p, err := FindAttributionPublication(ctx, tx, current.SourcePublicationId, true)
			if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
				return err
			}
			if err == nil {
				publication = p
			}
		}
	}
	if !matchesCompositionItems(in.Design.DesignData, in.Items) &&
		(in.Source == "design_square" || !matchesLegacyCheckoutItems(in.Design.DesignData, in.Items)) {
		return common.NewBizError(40915, "下单用料与当前设计不一致，请重新确认设计")
	}
	in.CreatorId = ""
	in.RewardPublicationId, in.RewardDesignId = 0, 0
	// Copies that change style are new work. Their historical origin can stay
	// on the draft, but this checkout must not earn for the old style.
	if publication != nil {
		hash, err := CompositionFingerprint(in.Design.DesignData)
		if err == nil && hash == publication.CompositionHash && publication.RewardCreatorId != in.UserId {
			root := publication
			if publication.RewardPublicationId > 0 {
				var rootErr error
				root, rootErr = FindAttributionPublication(ctx, tx, publication.RewardPublicationId, true)
				if rootErr != nil && !errors.Is(rootErr, sqlx.ErrNotFound) {
					return rootErr
				}
				if rootErr != nil {
					return nil
				}
			}
			if root.RewardCreatorId != "" && root.RewardCreatorId == publication.RewardCreatorId && root.CompositionHash == hash {
				in.CreatorId = root.RewardCreatorId
				in.RewardPublicationId = root.Id
				in.RewardDesignId = root.DesignId
			}
		}
	}
	return nil
}

func publicationSnapshotForReward(raw string) (int64, int64) {
	var snapshot struct {
		RewardPublicationId int64 `json:"rewardPublicationId"`
		RewardDesignId      int64 `json:"rewardDesignId"`
	}
	if json.Unmarshal([]byte(raw), &snapshot) != nil {
		return 0, 0
	}
	return snapshot.RewardPublicationId, snapshot.RewardDesignId
}
