package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/diy-service/internal/model"
	"github.com/askxuan/diy-service/internal/svc"
	"github.com/askxuan/diy-service/internal/types"
	"math"
	"strconv"
	"strings"
)

type DesignStudio struct {
	ctx context.Context
	svc *svc.ServiceContext
}

func NewDesignStudio(ctx context.Context, s *svc.ServiceContext) *DesignStudio {
	return &DesignStudio{ctx, s}
}
func studioUser(ctx context.Context) string {
	id := middleware.UserIDFromCtx(ctx)
	if id < 1 {
		return ""
	}
	return strconv.FormatInt(id, 10)
}
func canReadDesign(d *model.DiyDesign, user string) bool {
	return d != nil && (d.Status == model.DesignStatusPublic || (user != "" && d.UserId == user))
}

type studioBead struct {
	SlotId       string  `json:"slotId"`
	Position     int     `json:"position"`
	MaterialId   int64   `json:"materialId"`
	MaterialName string  `json:"materialName"`
	Spec         string  `json:"spec"`
	UnitPrice    float64 `json:"unitPrice"`
	Subtype      string  `json:"subtype"`
	Image        string  `json:"image"`
	DiameterMm   float64 `json:"diameterMm"`
	ColorHex     string  `json:"colorHex"`
	FiveElements string  `json:"fiveElements"`
	MaterialType string  `json:"materialType"`
	Shape        string  `json:"shape"`
	TextureKey   string  `json:"textureKey"`
	Finish       string  `json:"finish"`
	Translucency float64 `json:"translucency"`
	RenderAssets string  `json:"renderAssets,omitempty"`
}
type studioDocument struct {
	Version        int                  `json:"version"`
	WristSizeMm    float64              `json:"wristSizeMm"`
	FitAllowanceMm float64              `json:"fitAllowanceMm"`
	Beads          []studioBead         `json:"beads"`
	Cord           *types.DiyOrderItem  `json:"cord"`
	Items          []types.DiyOrderItem `json:"items"`
}

func decodeStudioDocument(raw string) (studioDocument, error) {
	var doc studioDocument
	if len(raw) > 128*1024 || json.Unmarshal([]byte(raw), &doc) != nil {
		return doc, common.ErrParam
	}
	if doc.Version != 2 || len(doc.Beads) < 1 || len(doc.Beads) > 60 || doc.WristSizeMm < 100 || doc.WristSizeMm > 260 || doc.FitAllowanceMm < 0 || doc.FitAllowanceMm > 30 {
		return doc, common.NewBizError(40014, "请检查珠子数量、手围与松量")
	}
	for _, b := range doc.Beads {
		if b.MaterialId < 1 || len(b.Spec) > 100 {
			return doc, common.ErrParam
		}
	}
	return doc, nil
}

// Canonicalize the bill and appearance from the catalog, not client-supplied prices,
// image URLs or item counts. The bead sequence is the single source of quantities.
func (l *DesignStudio) canonical(raw string) (string, float64, error) {
	doc, err := decodeStudioDocument(raw)
	if err != nil {
		return "", 0, err
	}
	mats := map[int64]*model.Material{}
	skus := map[int64][]*model.MaterialSku{}
	resolve := func(id int64, spec string) (*model.Material, float64, error) {
		if mats[id] == nil {
			m, e := l.svc.MaterialModel.FindOne(l.ctx, id)
			if e != nil {
				return nil, 0, common.NewBizError(40014, "材料不存在，请重新选择")
			}
			mats[id] = m
			v, e := l.svc.MaterialSkuModel.ListByMaterialId(l.ctx, id)
			if e != nil {
				return nil, 0, common.ErrSystem
			}
			skus[id] = v
		}
		m := mats[id]
		for _, sku := range skus[id] {
			if sku.Spec == spec {
				return m, sku.Price, nil
			}
		}
		if len(skus[id]) == 0 && m.Spec == spec {
			return m, m.UnitPrice, nil
		}
		return nil, 0, common.NewBizError(40014, "材料规格已变化，请重新选择")
	}
	doc.Items = []types.DiyOrderItem{}
	keys := map[string]int{}
	total := 0.0
	add := func(it types.DiyOrderItem) {
		key := fmt.Sprintf("%d|%s", it.MaterialId, it.Spec)
		if i, ok := keys[key]; ok {
			doc.Items[i].Quantity += it.Quantity
		} else {
			keys[key] = len(doc.Items)
			doc.Items = append(doc.Items, it)
		}
		total += it.UnitPrice * float64(it.Quantity)
	}
	for i, b := range doc.Beads {
		m, price, e := resolve(b.MaterialId, b.Spec)
		if e != nil {
			return "", 0, e
		}
		if m.Category == "cord" {
			return "", 0, common.ErrParam
		}
		doc.Beads[i] = studioBead{SlotId: fmt.Sprintf("bead-%d", i), Position: i, MaterialId: m.Id, MaterialName: m.Name, Spec: b.Spec, UnitPrice: price, Subtype: m.Category, Image: m.Image, DiameterMm: m.DiameterMm, ColorHex: m.ColorHex, FiveElements: m.FiveElements, MaterialType: m.MaterialType, Shape: m.Shape, TextureKey: m.TextureKey, Finish: m.Finish, Translucency: m.Translucency, RenderAssets: m.RenderAssets}
		add(types.DiyOrderItem{MaterialId: m.Id, MaterialName: m.Name, Spec: b.Spec, UnitPrice: price, Quantity: 1, Subtype: m.Category})
	}
	if doc.Cord != nil {
		m, price, e := resolve(doc.Cord.MaterialId, doc.Cord.Spec)
		if e != nil {
			return "", 0, e
		}
		if m.Category != "cord" {
			return "", 0, common.ErrParam
		}
		doc.Cord = &types.DiyOrderItem{MaterialId: m.Id, MaterialName: m.Name, Spec: doc.Cord.Spec, UnitPrice: price, Quantity: 1, Subtype: "cord"}
		add(*doc.Cord)
	}
	out, e := json.Marshal(doc)
	return string(out), math.Round(total*100) / 100, e
}

func (l *DesignStudio) Save(req *types.DesignSaveReq) (*types.DesignSaveResp, error) {
	user := studioUser(l.ctx)
	if user == "" {
		return nil, common.ErrUnauthorized
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len([]rune(name)) > 60 || len([]rune(req.Description)) > 600 {
		return nil, common.ErrParam
	}
	var previous *model.DiyDesign
	if req.Id > 0 {
		d, e := l.svc.DiyDesignModel.FindOne(l.ctx, req.Id)
		if e != nil || d.UserId != user {
			return nil, ErrDesignNotFound
		}
		if req.Revision < 1 {
			return nil, common.ErrParam
		}
		previous = d
	}
	raw, price, err := l.canonical(req.DesignData)
	if err != nil {
		return nil, err
	}
	d := &model.DiyDesign{Id: req.Id, UserId: user, Name: name, DesignData: raw, TotalPrice: price, Status: model.DesignStatusPrivate, Description: strings.TrimSpace(req.Description)}
	if previous != nil {
		d.SourceDesignId = previous.SourceDesignId
		err = l.svc.DiyDesignModel.UpdateOwned(l.ctx, d, req.Revision)
	} else {
		d, err = l.svc.DiyDesignModel.Insert(l.ctx, d)
	}
	if err != nil {
		return nil, err
	}
	return &types.DesignSaveResp{Id: d.Id, Revision: d.Revision}, nil
}

func (l *DesignStudio) ChangeStatus(id, revision int64, status string, admin bool) (*types.DiyDesign, error) {
	user := studioUser(l.ctx)
	if user == "" {
		return nil, common.ErrUnauthorized
	}
	d, err := l.svc.DiyDesignModel.FindOne(l.ctx, id)
	if err != nil {
		return nil, ErrDesignNotFound
	}
	if !admin && d.UserId != user {
		return nil, ErrDesignNotFound
	}
	if status != model.DesignStatusPublic && status != model.DesignStatusPrivate && !(admin && status == model.DesignStatusRejected) {
		return nil, common.ErrParam
	}
	if !admin && d.Status == model.DesignStatusRejected && status == model.DesignStatusPublic {
		return nil, common.NewBizError(40014, "请修改被下架的作品后重新发布")
	}
	if revision < 1 {
		return nil, common.ErrParam
	}
	if status == model.DesignStatusPublic {
		// Stock and current prices are checked before listing; final checkout still
		// checks them again transactionally, without reserving inventory here.
		items, e := parseDesignOrderItems(d.DesignData)
		if e != nil || len(items) == 0 {
			return nil, common.ErrParam
		}
		availability, e := model.CheckPricedOrderItems(l.ctx, l.svc.DB, toPricedInputs(items))
		if e != nil {
			return nil, common.ErrSystem
		}
		if !availability.Orderable {
			return nil, common.NewBizError(40014, "部分材料缺货或已下架，请替换后发布")
		}
		if doc, e := decodeStudioDocument(d.DesignData); e == nil && doc.Version == 2 {
			raw, price, e := l.canonical(d.DesignData)
			if e != nil {
				return nil, e
			}
			d.DesignData, d.TotalPrice = raw, price
		} else {
			d.TotalPrice = availability.MaterialFee
		}
	}
	d.Status = status
	if err = l.svc.DiyDesignModel.UpdateOwned(l.ctx, d, revision); err != nil {
		return nil, err
	}
	t := toTypesDesign(d)
	return &t, nil
}

func (l *DesignStudio) Copy(id int64) (*types.DiyDesign, error) {
	user := studioUser(l.ctx)
	if user == "" {
		return nil, common.ErrUnauthorized
	}
	source, e := l.svc.DiyDesignModel.FindOne(l.ctx, id)
	if e != nil || !canReadDesign(source, user) {
		return nil, ErrDesignNotFound
	}
	name := []rune(source.Name)
	if len(name) > 55 {
		name = name[:55]
	}
	d, e := l.svc.DiyDesignModel.Insert(l.ctx, &model.DiyDesign{UserId: user, Name: string(name) + " · 副本", DesignData: source.DesignData, TotalPrice: source.TotalPrice, Status: model.DesignStatusPrivate, SourceDesignId: source.Id, Description: source.Description})
	if e != nil {
		return nil, common.ErrSystem
	}
	t := toTypesDesign(d)
	return &t, nil
}
