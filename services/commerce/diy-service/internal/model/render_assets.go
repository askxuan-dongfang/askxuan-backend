package model

import (
	"encoding/json"
	"math"
	"net/url"
	"strings"
)

// Separate cutout photographs from UV textures; a product catalog photograph
// must not accidentally be wrapped around every 3D bead.
type ImageCrop struct {
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Width       float64 `json:"width"`
	Height      float64 `json:"height"`
	ImageWidth  float64 `json:"imageWidth"`
	ImageHeight float64 `json:"imageHeight"`
}
type RenderAssets struct {
	Studio          *MaterialStudio `json:"studio,omitempty"`
	ImageCrop       *ImageCrop      `json:"imageCrop,omitempty"`
	BeadImageURL    string          `json:"beadImageUrl,omitempty"`
	AlbedoMapURL    string          `json:"albedoMapUrl,omitempty"`
	NormalMapURL    string          `json:"normalMapUrl,omitempty"`
	RoughnessMapURL string          `json:"roughnessMapUrl,omitempty"`
	Source          string          `json:"source,omitempty"`
	Attribution     string          `json:"attribution,omitempty"`
}

type GeneratedMaterialModel struct {
	JobID   string `json:"jobId"`
	GLBURL  string `json:"glbUrl"`
	USDZURL string `json:"usdzUrl"`
}
type MaterialStudio struct {
	Version          int                     `json:"version"`
	Part             string                  `json:"part"`
	Motion           string                  `json:"motion"`
	Source           string                  `json:"source"`
	TemplateID       string                  `json:"templateId"`
	DimensionsMM     []float64               `json:"dimensionsMm"`
	Rotation         []float64               `json:"rotation"`
	Mount            []float64               `json:"mount"`
	ConnectorMM      float64                 `json:"connectorMm"`
	RigidFraction    float64                 `json:"rigidFraction"`
	Photos           []string                `json:"photos"`
	Model            *GeneratedMaterialModel `json:"model,omitempty"`
	PreviewConfirmed bool                    `json:"previewConfirmed"`
}

func StudioAssets(raw string) *MaterialStudio {
	var a RenderAssets
	if json.Unmarshal([]byte(raw), &a) != nil {
		return nil
	}
	return a.Studio
}

func validVector(v []float64, lo, hi float64) bool {
	if len(v) != 3 {
		return false
	}
	for _, n := range v {
		if math.IsNaN(n) || math.IsInf(n, 0) || n < lo || n > hi {
			return false
		}
	}
	return true
}
func ValidAssetURL(v string) bool {
	if len(v) > 2048 || strings.ContainsAny(v, "\r\n\\") {
		return false
	}
	if strings.HasPrefix(v, "/") && !strings.HasPrefix(v, "//") {
		return true
	}
	u, e := url.Parse(v)
	return e == nil && u.Scheme == "https" && u.Host != "" && u.User == nil
}
func ValidMaterialStudio(s *MaterialStudio) bool {
	if s == nil {
		return true
	}
	if s.Version != 1 || !validVector(s.DimensionsMM, .5, 120) || !validVector(s.Rotation, -180, 180) || !validVector(s.Mount, -.5, .5) || len(s.Photos) > 4 {
		return false
	}
	if s.Part == "bead" {
		if s.Motion != "fixed" || s.ConnectorMM != 0 {
			return false
		}
	} else if s.Part == "hanging" {
		if (s.Motion != "rigid" && s.Motion != "flexible") || s.ConnectorMM < 1 || s.ConnectorMM > 30 || s.Mount[2] >= .5 {
			return false
		}
	} else {
		return false
	}
	if math.IsNaN(s.ConnectorMM) || math.IsInf(s.ConnectorMM, 0) || math.IsNaN(s.RigidFraction) || math.IsInf(s.RigidFraction, 0) || s.RigidFraction < .1 || s.RigidFraction > .9 {
		return false
	}
	for _, p := range s.Photos {
		if !ValidAssetURL(p) {
			return false
		}
	}
	if s.Source == "generated" {
		return s.TemplateID == "custom" && (s.Model == nil || (len(s.Model.JobID) >= 16 && len(s.Model.JobID) <= 64 && ValidAssetURL(s.Model.GLBURL) && ValidAssetURL(s.Model.USDZURL)))
	}
	if s.Source != "template" || s.Model != nil {
		return false
	}
	switch s.TemplateID {
	case "round", "disc", "barrel", "faceted", "three_way", "buddha_head", "pendant_generic", "pendant_lotus", "pendant_gourd", "pendant_peace", "pendant_woodfish", "tassel", "tassel_knot":
		return true
	}
	return false
}

func ValidRenderAssets(raw string) bool {
	if raw == "" {
		return true
	}
	if len(raw) > 16384 {
		return false
	}
	var a RenderAssets
	if json.Unmarshal([]byte(raw), &a) != nil {
		return false
	}
	if !ValidMaterialStudio(a.Studio) {
		return false
	}
	if a.Source != "" && a.Source != "photograph" && a.Source != "procedural" && a.Source != "licensed" {
		return false
	}
	if c := a.ImageCrop; c != nil && (c.X < 0 || c.Y < 0 || c.Width <= 0 || c.Height <= 0 || c.X+c.Width > c.ImageWidth || c.Y+c.Height > c.ImageHeight || c.ImageWidth > 16384 || c.ImageHeight > 16384) {
		return false
	}
	if len([]rune(a.Attribution)) > 400 {
		return false
	}
	for _, v := range []string{a.BeadImageURL, a.AlbedoMapURL, a.NormalMapURL, a.RoughnessMapURL} {
		if v == "" {
			continue
		}
		if !ValidAssetURL(v) {
			return false
		}
	}
	return true
}
