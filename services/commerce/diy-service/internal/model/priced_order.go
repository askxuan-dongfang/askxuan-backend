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

func resolvePricedItem(ctx context.Context, session sqlx.Session, requested PricedOrderItemInput) (*DiyOrderItem, float64, error) {
	if requested.MaterialId <= 0 || requested.Quantity <= 0 {
		return nil, 0, ErrOrderPricingInvalid
	}
	var material Material
	if err := session.QueryRowCtx(ctx, &material, `SELECT id,name,spec,unit_price,unit,category,five_elements,image,stock,status FROM material WHERE id=? FOR UPDATE`, requested.MaterialId); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, 0, ErrOrderMaterialUnavailable
		}
		return nil, 0, err
	}
	if material.Status != MaterialStatusOnShelf {
		return nil, 0, ErrOrderMaterialUnavailable
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
		return nil, 0, ErrOrderMaterialUnavailable
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
