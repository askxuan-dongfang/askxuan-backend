package logic

import (
	"bytes"
	"encoding/base64"
	"errors"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"strings"
)

// Provider limits are advisory. Enforce decoded budgets before a mobile renderer
// receives a model, including accessors with no backing buffer (zero allocation).
func validateGLBBudget(doc map[string]any, bin []byte) error {
	bad := errors.New("GLB exceeds geometry or texture budget")
	integer := func(v any, limit int) (int, bool) {
		n, ok := v.(float64)
		return int(n), ok && n >= 0 && n <= float64(limit) && math.Trunc(n) == n
	}
	index := func(v any, count int) (int, bool) {
		n, ok := integer(v, count-1)
		return n, ok && count > 0
	}
	// Compressed geometry and instancing can expand beyond manifest counts.
	for _, key := range []string{"extensionsUsed", "extensionsRequired"} {
		for _, ext := range array(doc[key]) {
			if ext == "KHR_draco_mesh_compression" || ext == "EXT_meshopt_compression" || ext == "EXT_mesh_gpu_instancing" {
				return bad
			}
		}
	}
	accessors, meshes, nodes := array(doc["accessors"]), array(doc["meshes"]), array(doc["nodes"])
	if len(accessors) > 512 || len(meshes) == 0 || len(meshes) > 64 || len(nodes) > 128 || len(array(doc["materials"])) > 32 {
		return bad
	}
	counts := make([]int, len(accessors))
	totalAccessors := 0
	for i, a := range accessors {
		m, ok := a.(map[string]any)
		if !ok || m["sparse"] != nil {
			return bad
		}
		count, ok := integer(m["count"], 300000)
		if !ok || count == 0 {
			return bad
		}
		components := map[string]int{"SCALAR": 1, "VEC2": 2, "VEC3": 3, "VEC4": 4, "MAT2": 4, "MAT3": 9, "MAT4": 16}
		kind, _ := m["type"].(string)
		width := components[kind]
		if width == 0 {
			return bad
		}
		totalAccessors += count * width
		if totalAccessors > 2000000 {
			return bad
		}
		counts[i] = count
	}
	meshVertices := make([]int, len(meshes))
	for i, raw := range meshes {
		mesh, ok := raw.(map[string]any)
		if !ok || len(array(mesh["primitives"])) == 0 || len(array(mesh["primitives"])) > 64 {
			return bad
		}
		for _, p := range array(mesh["primitives"]) {
			primitive, ok := p.(map[string]any)
			if !ok || len(array(primitive["targets"])) != 0 {
				return bad
			}
			attrs, ok := primitive["attributes"].(map[string]any)
			if !ok {
				return bad
			}
			pos, ok := index(attrs["POSITION"], len(accessors))
			if !ok || accessors[pos].(map[string]any)["type"] != "VEC3" {
				return bad
			}
			meshVertices[i] += counts[pos]
			if meshVertices[i] > 100000 {
				return bad
			}
		}
	}
	totalVertices := 0
	for _, count := range meshVertices {
		totalVertices += count
	}
	if totalVertices > 100000 {
		return bad
	}
	totalVertices = 0
	parents := make([]int, len(nodes))
	for _, raw := range nodes {
		node, ok := raw.(map[string]any)
		if !ok {
			return bad
		}
		if mesh, exists := node["mesh"]; exists {
			i, ok := index(mesh, len(meshes))
			if !ok {
				return bad
			}
			totalVertices += meshVertices[i]
		}
		for _, child := range array(node["children"]) {
			i, ok := index(child, len(nodes))
			if !ok || parents[i] != 0 {
				return bad
			}
			parents[i]++
		}
	}
	if len(nodes) == 0 {
		for _, count := range meshVertices {
			totalVertices += count
		}
	}
	if totalVertices > 100000 {
		return bad
	}
	// Cycles otherwise recurse indefinitely in loaders even without a scene.
	colors := make([]byte, len(nodes))
	var visit func(int) bool
	visit = func(i int) bool {
		if colors[i] == 1 {
			return false
		}
		if colors[i] == 2 {
			return true
		}
		colors[i] = 1
		for _, child := range array(nodes[i].(map[string]any)["children"]) {
			j, _ := index(child, len(nodes))
			if !visit(j) {
				return false
			}
		}
		colors[i] = 2
		return true
	}
	for i := range nodes {
		if !visit(i) {
			return bad
		}
	}
	for _, raw := range array(doc["scenes"]) {
		scene, ok := raw.(map[string]any)
		if !ok {
			return bad
		}
		seen := map[int]bool{}
		for _, rawIndex := range array(scene["nodes"]) {
			i, ok := index(rawIndex, len(nodes))
			if !ok || parents[i] != 0 || seen[i] {
				return bad
			}
			seen[i] = true
		}
	}
	views := array(doc["bufferViews"])
	images := array(doc["images"])
	if len(views) > 1024 || len(images) > 16 || len(array(doc["textures"])) > 16 {
		return bad
	}
	pixels := 0
	for _, raw := range images {
		im, ok := raw.(map[string]any)
		if !ok {
			return bad
		}
		var data []byte
		if uri, ok := im["uri"].(string); ok {
			parts := strings.SplitN(uri, ",", 2)
			if len(parts) != 2 {
				return bad
			}
			var err error
			data, err = base64.StdEncoding.DecodeString(parts[1])
			if err != nil {
				return bad
			}
		} else {
			i, ok := index(im["bufferView"], len(views))
			if !ok {
				return bad
			}
			view, ok := views[i].(map[string]any)
			if !ok {
				return bad
			}
			buffer, ok := integer(view["buffer"], 0)
			if !ok || buffer != 0 {
				return bad
			}
			offset := 0
			if v, exists := view["byteOffset"]; exists {
				offset, ok = integer(v, len(bin))
				if !ok {
					return bad
				}
			}
			size, ok := integer(view["byteLength"], len(bin))
			if !ok || offset+size > len(bin) {
				return bad
			}
			data = bin[offset : offset+size]
		}
		count, err := decodedTexturePixels(data)
		if err != nil {
			return bad
		}
		pixels += count
		if pixels > 16*1024*1024 {
			return bad
		}
	}
	return nil
}

func decodedTexturePixels(data []byte) (int, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 4096 || cfg.Height > 4096 {
		return 0, errors.New("invalid or oversized model texture")
	}
	return cfg.Width * cfg.Height, nil
}

func array(v any) []any { a, _ := v.([]any); return a }
