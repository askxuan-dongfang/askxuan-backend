package model

import (
	"encoding/json"
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
	ImageCrop       *ImageCrop `json:"imageCrop,omitempty"`
	BeadImageURL    string     `json:"beadImageUrl,omitempty"`
	AlbedoMapURL    string     `json:"albedoMapUrl,omitempty"`
	NormalMapURL    string     `json:"normalMapUrl,omitempty"`
	RoughnessMapURL string     `json:"roughnessMapUrl,omitempty"`
	Source          string     `json:"source,omitempty"`
	Attribution     string     `json:"attribution,omitempty"`
}

func ValidRenderAssets(raw string) bool {
	if raw == "" {
		return true
	}
	if len(raw) > 8192 {
		return false
	}
	var a RenderAssets
	if json.Unmarshal([]byte(raw), &a) != nil {
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
		if len(v) > 2048 || strings.ContainsAny(v, "\r\n\\") {
			return false
		}
		if strings.HasPrefix(v, "/") && !strings.HasPrefix(v, "//") {
			continue
		}
		u, err := url.Parse(v)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
			return false
		}
	}
	return true
}
