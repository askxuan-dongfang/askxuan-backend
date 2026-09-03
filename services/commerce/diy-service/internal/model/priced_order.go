package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var (
	ErrOrderMaterialUnavailable = errors.New("order material unavailable")
	ErrOrderStockInsufficient   = errors.New("order stock insufficient")
	ErrOrderBlessingUnavailable = errors.New("order blessing service unavailable")
	ErrOrderPricingInvalid      = errors.New("order pricing invalid")
)

type PricedOrderItemInput struct {
	MaterialId        int64
	Spec              string
	Quantity          int
	Subtype           string
	SnapshotUnitPrice float64
}

type PricedOrderInput struct {
	UserId           string
	Design           *DiyDesign
	Items            []PricedOrderItemInput
	BlessServiceCode string
	AddressId        int64
	Source           string
	CreatorId        string
}

type PricedOrderResult struct {
	Order *DiyOrder
	Items []*DiyOrderItem
}

type PricedOrderAvailabilityIssue struct {
	MaterialId   int64
	MaterialName string
	Spec         string
	Quantity     int
	Reason       string
}

type PricedOrderAvailability struct {
	Orderable           bool
	MaterialFee         float64
	OriginalMaterialFee float64
	PriceChanged        bool
	Issues              []PricedOrderAvailabilityIssue
}

type OrderMaterialUnavailableError struct {
	MaterialId   int64
	MaterialName string
	Spec         string
	Reason       string
}

func (e *OrderMaterialUnavailableError) Error() string {
	return fmt.Sprintf("%s: materialId=%d name=%q spec=%q", e.Reason, e.MaterialId, e.MaterialName, e.Spec)
}

func (e *OrderMaterialUnavailableError) Unwrap() error { return ErrOrderMaterialUnavailable }

type pricingSnapshot struct {
	OriginalMaterialFee float64         `json:"originalMaterialFee"`
	MaterialFee         float64         `json:"materialFee"`
	BlessFee            float64         `json:"blessFee"`
	TotalFee            float64         `json:"totalFee"`
	PriceChanged        bool            `json:"priceChanged"`
	CreatorShareRate    float64         `json:"creatorShareRate"`
	Items               []*DiyOrderItem `json:"items"`
}

func CreatePricedOrder(ctx context.Context, conn sqlx.SqlConn, orderModel DiyOrderModel, itemModel DiyOrderItemModel, in PricedOrderInput) (result *PricedOrderResult, err error) {
	if in.Design == nil || len(in.Items) == 0 {
		return nil, ErrOrderPricingInvalid
	}

	err = conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		items := make([]*DiyOrderItem, 0, len(in.Items))
		var materialFee, originalMaterialFee float64
		for _, requested := range in.Items {
			item, original, resolveErr := resolvePricedItem(ctx, session, requested)
			if resolveErr != nil {
				return resolveErr
			}
			items = append(items, item)
			materialFee = roundMoney(materialFee + item.UnitPrice*float64(item.Quantity))
			originalMaterialFee = roundMoney(originalMaterialFee + original*float64(item.Quantity))
		}
		if originalMaterialFee == 0 && in.Design.TotalPrice > 0 {
			originalMaterialFee = roundMoney(in.Design.TotalPrice)
		}

		blessFee, blessErr := resolveBlessFee(ctx, session, in.BlessServiceCode)
		if blessErr != nil {
			return blessErr
		}
		shareRate, shareErr := resolveCreatorShareRate(ctx, session, in.CreatorId)
		if shareErr != nil {
			return shareErr
		}
		priceChanged := math.Abs(originalMaterialFee-materialFee) >= 0.005
		totalFee := roundMoney(materialFee + blessFee)

		designSnapshot, marshalErr := json.Marshal(in.Design)
		if marshalErr != nil {
			return marshalErr
		}
		priceSnapshot, marshalErr := json.Marshal(pricingSnapshot{
			OriginalMaterialFee: originalMaterialFee,
			MaterialFee:         materialFee,
			BlessFee:            blessFee,
			TotalFee:            totalFee,
			PriceChanged:        priceChanged,
			CreatorShareRate:    shareRate,
			Items:               items,
		})
		if marshalErr != nil {
			return marshalErr
		}

		priceChangedValue := 0
		if priceChanged {
			priceChangedValue = 1
		}
		order, insertErr := orderModel.InsertSession(ctx, session, &DiyOrder{
			UserId:              in.UserId,
			DesignId:            in.Design.Id,
			MaterialFee:         materialFee,
			BlessFee:            blessFee,
			TotalFee:            totalFee,
			Status:              DiyStatusPendingReview,
			AddressId:           in.AddressId,
			Source:              in.Source,
			CreatorId:           in.CreatorId,
			CreatorShareRate:    shareRate,
			OriginalMaterialFee: originalMaterialFee,
			PriceChanged:        priceChangedValue,
			DesignSnapshot:      string(designSnapshot),
			PricingSnapshot:     string(priceSnapshot),
		})
		if insertErr != nil {
			return insertErr
		}
		for _, item := range items {
			item.OrderId = order.Id
			if _, insertErr = itemModel.InsertSession(ctx, session, item); insertErr != nil {
				return insertErr
			}
		}
		result = &PricedOrderResult{Order: order, Items: items}
		return nil
	})
	return result, err
}

// CheckPricedOrderItems performs the same catalog/SKU/stock checks used at
// checkout, without locking rows or changing inventory.
func CheckPricedOrderItems(ctx context.Context, conn sqlx.SqlConn, inputs []PricedOrderItemInput) (*PricedOrderAvailability, error) {
	result := &PricedOrderAvailability{Issues: make([]PricedOrderAvailabilityIssue, 0)}
	if len(inputs) == 0 {
		return result, ErrOrderPricingInvalid
	}

	type aggregate struct {
		input    PricedOrderItemInput
		quantity int
	}
	order := make([]string, 0, len(inputs))
	grouped := make(map[string]*aggregate, len(inputs))
	for _, input := range inputs {
		if input.MaterialId <= 0 || input.Quantity <= 0 {
			return result, ErrOrderPricingInvalid
		}
		key := fmt.Sprintf("%d|%s", input.MaterialId, strings.TrimSpace(input.Spec))
		if current := grouped[key]; current != nil {
			current.quantity += input.Quantity
			continue
		}
		copy := input
		grouped[key] = &aggregate{input: copy, quantity: input.Quantity}
		order = append(order, key)
	}

	for _, key := range order {
		entry := grouped[key]
		input := entry.input
		quantity := entry.quantity
		name := fmt.Sprintf("材料 #%d", input.MaterialId)
		spec := strings.TrimSpace(input.Spec)
		var material struct {
			Id        int64   `db:"id"`
			Name      string  `db:"name"`
			Spec      string  `db:"spec"`
			UnitPrice float64 `db:"unit_price"`
			Stock     int     `db:"stock"`
			Status    string  `db:"status"`
		}
		if err := conn.QueryRowCtx(ctx, &material, `SELECT id,name,spec,unit_price,stock,status FROM material WHERE id=?`, input.MaterialId); err != nil {
			if errors.Is(err, sqlx.ErrNotFound) {
				result.Issues = append(result.Issues, PricedOrderAvailabilityIssue{MaterialId: input.MaterialId, MaterialName: name, Spec: spec, Quantity: quantity, Reason: "not_found"})
				continue
			}
			return nil, err
		}
		name = material.Name
		if spec == "" {
			spec = material.Spec
		}
		if material.Status != MaterialStatusOnShelf {
			result.Issues = append(result.Issues, PricedOrderAvailabilityIssue{MaterialId: material.Id, MaterialName: name, Spec: spec, Quantity: quantity, Reason: "off_shelf"})
			continue
		}

		unitPrice := material.UnitPrice
		stock := material.Stock
		var sku MaterialSku
		err := conn.QueryRowCtx(ctx, &sku, `SELECT id,material_id,spec,price,stock FROM material_sku WHERE material_id=? AND spec=?`, material.Id, spec)
		if err == nil {
			unitPrice = sku.Price
			stock = sku.Stock
		} else if !errors.Is(err, sqlx.ErrNotFound) {
			return nil, err
		} else if spec != material.Spec {
			result.Issues = append(result.Issues, PricedOrderAvailabilityIssue{MaterialId: material.Id, MaterialName: name, Spec: spec, Quantity: quantity, Reason: "spec_unavailable"})
			continue
		}
		if stock < quantity || material.Stock < quantity {
			result.Issues = append(result.Issues, PricedOrderAvailabilityIssue{MaterialId: material.Id, MaterialName: name, Spec: spec, Quantity: quantity, Reason: "stock_insufficient"})
			continue
		}
		result.MaterialFee = roundMoney(result.MaterialFee + unitPrice*float64(quantity))
		original := input.SnapshotUnitPrice
		if original < 0 {
			original = 0
		}
		result.OriginalMaterialFee = roundMoney(result.OriginalMaterialFee + original*float64(quantity))
	}
	result.Orderable = len(result.Issues) == 0
	result.PriceChanged = result.Orderable && math.Abs(result.OriginalMaterialFee-result.MaterialFee) >= 0.005
	return result, nil
}

func resolvePricedItem(ctx context.Context, session sqlx.Session, requested PricedOrderItemInput) (*DiyOrderItem, float64, error) {
	if requested.MaterialId <= 0 || requested.Quantity <= 0 {
		return nil, 0, ErrOrderPricingInvalid
	}
	// Pricing only needs the inventory contract. Keep it independent from
	// presentation fields that can evolve without changing order locking SQL.
	var material struct {
		Id        int64   `db:"id"`
		Name      string  `db:"name"`
		Spec      string  `db:"spec"`
		UnitPrice float64 `db:"unit_price"`
		Unit      string  `db:"unit"`
		Category  string  `db:"category"`
		Stock     int     `db:"stock"`
		Status    string  `db:"status"`
	}
	if err := session.QueryRowCtx(ctx, &material, `SELECT id,name,spec,unit_price,unit,category,stock,status FROM material WHERE id=? FOR UPDATE`, requested.MaterialId); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, 0, &OrderMaterialUnavailableError{MaterialId: requested.MaterialId, Spec: requested.Spec, Reason: "not_found"}
		}
		return nil, 0, err
	}
	if material.Status != MaterialStatusOnShelf {
		return nil, 0, &OrderMaterialUnavailableError{MaterialId: material.Id, MaterialName: material.Name, Spec: requested.Spec, Reason: "off_shelf"}
	}

	spec := strings.TrimSpace(requested.Spec)
	if spec == "" {
		spec = material.Spec
	}
	unitPrice := material.UnitPrice
	stock := material.Stock
	var sku MaterialSku
	err := session.QueryRowCtx(ctx, &sku, `SELECT id,material_id,spec,price,stock FROM material_sku WHERE material_id=? AND spec=? FOR UPDATE`, material.Id, spec)
	if err == nil {
		unitPrice = sku.Price
		stock = sku.Stock
	} else if !errors.Is(err, sqlx.ErrNotFound) {
		return nil, 0, err
	} else if spec != material.Spec {
		return nil, 0, &OrderMaterialUnavailableError{MaterialId: material.Id, MaterialName: material.Name, Spec: spec, Reason: "spec_unavailable"}
	}
	if stock < requested.Quantity || material.Stock < requested.Quantity {
		return nil, 0, ErrOrderStockInsufficient
	}

	updated, err := session.ExecCtx(ctx, `UPDATE material SET stock=stock-? WHERE id=? AND status=? AND stock>=?`, requested.Quantity, material.Id, MaterialStatusOnShelf, requested.Quantity)
	if err != nil {
		return nil, 0, err
	}
	rows, err := updated.RowsAffected()
	if err != nil || rows != 1 {
		return nil, 0, ErrOrderStockInsufficient
	}
	if sku.Id > 0 {
		updated, err = session.ExecCtx(ctx, `UPDATE material_sku SET stock=stock-? WHERE id=? AND stock>=?`, requested.Quantity, sku.Id, requested.Quantity)
		if err != nil {
			return nil, 0, err
		}
		rows, err = updated.RowsAffected()
		if err != nil || rows != 1 {
			return nil, 0, ErrOrderStockInsufficient
		}
	}

	original := requested.SnapshotUnitPrice
	if original < 0 {
		original = 0
	}
	return &DiyOrderItem{
		MaterialId:   material.Id,
		SkuId:        sku.Id,
		MaterialName: material.Name,
		Spec:         spec,
		UnitPrice:    roundMoney(unitPrice),
		Quantity:     requested.Quantity,
		Subtype:      material.Category,
	}, original, nil
}

func resolveBlessFee(ctx context.Context, session sqlx.Session, code string) (float64, error) {
	if strings.TrimSpace(code) == "" {
		return 0, nil
	}
	var price float64
	if err := session.QueryRowCtx(ctx, &price, `SELECT price FROM extra_service WHERE code=? AND status=?`, code, BlessingServiceStatusOnShelf); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return 0, ErrOrderBlessingUnavailable
		}
		return 0, err
	}
	return roundMoney(price), nil
}

func resolveCreatorShareRate(ctx context.Context, session sqlx.Session, creatorId string) (float64, error) {
	if creatorId == "" {
		return 0, nil
	}
	var raw string
	if err := session.QueryRowCtx(ctx, &raw, `SELECT config_value FROM diy_config WHERE config_key='diy_design_creator_share'`); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return 0, nil
		}
		return 0, err
	}
	rate, err := strconv.ParseFloat(raw, 64)
	if err != nil || rate < 0 || rate > 1 {
		return 0, fmt.Errorf("invalid diy_design_creator_share %q", raw)
	}
	return rate, nil
}

func roundMoney(value float64) float64 {
	return math.Round(value*100) / 100
}

func earningNo(orderId int64) string {
	return fmt.Sprintf("DCE%s%06d", time.Now().Format("20060102"), orderId%1000000)
}
