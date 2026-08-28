package model

import (
	"reflect"
	"testing"
)

func TestNormalizeMaterialPresentationSupportsLegacyAdminPayload(t *testing.T) {
	material := &Material{Category: MaterialCategorySpacer, FiveElements: "金", Translucency: 2}
	normalizeMaterialPresentation(material)

	if material.FiveElements != "metal" || material.MaterialType != "metal" {
		t.Fatalf("unexpected material classification: %#v", material)
	}
	if material.Shape != "disc" || material.DiameterMm != 10 {
		t.Fatalf("unexpected fallback rendering: %#v", material)
	}
	if material.ColorHex != "#B9C2C4" || material.TextureKey != "plain" || material.Finish != "polished" {
		t.Fatalf("unexpected fallback appearance: %#v", material)
	}
	if material.Translucency != 1 {
		t.Fatalf("translucency was not clamped: %v", material.Translucency)
	}
}

func TestNormalizeMaterialPresentationKeepsConfiguredAppearance(t *testing.T) {
	material := &Material{
		Category: MaterialCategoryMainBead, FiveElements: "water", MaterialType: "crystal",
		Shape: "faceted", DiameterMm: 8, ColorHex: "#77ACC4", TextureKey: "crystal",
		Finish: "faceted", Translucency: 0.66,
	}
	normalizeMaterialPresentation(material)

	if material.MaterialType != "crystal" || material.Shape != "faceted" || material.DiameterMm != 8 || material.ColorHex != "#77ACC4" {
		t.Fatalf("configured appearance changed: %#v", material)
	}
}

func TestMaterialListFilterIncludesPublicShelfStatus(t *testing.T) {
	where, args := materialListFilter(MaterialCategoryMainBead, "玉", MaterialStatusOnShelf)
	if where != "1=1 AND category = ? AND name LIKE ? AND status = ?" {
		t.Fatalf("unexpected filter: %s", where)
	}
	want := []interface{}{MaterialCategoryMainBead, "%玉%", MaterialStatusOnShelf}
	if !reflect.DeepEqual(args, want) {
		t.Fatalf("unexpected args: %#v", args)
	}
}
