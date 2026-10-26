package model

import (
	"encoding/json"
	"testing"
)

func TestStudioMaterialValidation(t *testing.T) {
	base := MaterialStudio{Version: 1, Part: "hanging", Motion: "flexible", Source: "template", TemplateID: "tassel_knot", DimensionsMM: []float64{12, 4, 30}, Rotation: []float64{0, 0, 0}, Mount: []float64{0, 0, -.5}, ConnectorMM: 3, RigidFraction: .3, Photos: []string{"/bucket/temp/photo.png"}, PreviewConfirmed: true}
	if !ValidMaterialStudio(&base) {
		t.Fatal("valid hanging studio rejected")
	}
	for name, change := range map[string]func(*MaterialStudio){"bead_motion": func(s *MaterialStudio) { s.Part = "bead" }, "zero_free_end": func(s *MaterialStudio) { s.Mount = []float64{0, 0, .5} }, "invalid_dimension": func(s *MaterialStudio) { s.DimensionsMM = []float64{0, 1, 2} }, "unknown_template": func(s *MaterialStudio) { s.TemplateID = "unknown" }, "external_scheme": func(s *MaterialStudio) { s.Photos = []string{"file:///secret"} }, "incomplete_rotation": func(s *MaterialStudio) { s.Rotation = []float64{0} }, "long_connector": func(s *MaterialStudio) { s.ConnectorMM = 31 }} {
		t.Run(name, func(t *testing.T) {
			s := base
			change(&s)
			if ValidMaterialStudio(&s) {
				t.Fatal("invalid studio accepted")
			}
		})
	}
	draft := base
	draft.Source = "generated"
	draft.TemplateID = "custom"
	draft.Model = nil
	draft.PreviewConfirmed = false
	if !ValidMaterialStudio(&draft) {
		t.Fatal("generated draft must save before generation")
	}
	raw, _ := json.Marshal(RenderAssets{Studio: &base})
	if !ValidRenderAssets(string(raw)) || StudioAssets(string(raw)) == nil {
		t.Fatal("studio JSON roundtrip failed")
	}
	if !ValidRenderAssets(`{"albedoMapUrl":"/existing.jpg"}`) || StudioAssets(`{"albedoMapUrl":"/existing.jpg"}`) != nil {
		t.Fatal("legacy presentation changed")
	}
}
