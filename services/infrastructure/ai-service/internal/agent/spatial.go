package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
)

type EvidenceItem struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Detail string `json:"detail,omitempty"`
}
type EvidenceChart struct {
	Kind     string         `json:"kind"`
	Title    string         `json:"title"`
	Note     string         `json:"note"`
	Items    []EvidenceItem `json:"items,omitempty"`
	Columns  []string       `json:"columns,omitempty"`
	Rows     [][]string     `json:"rows,omitempty"`
	Geometry *Plan          `json:"geometry,omitempty"`
}
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
type Room struct {
	Name   string  `json:"name"`
	Points []Point `json:"points"`
}
type Plan struct {
	Confirmed     bool    `json:"confirmed"`
	MetersPerUnit float64 `json:"metersPerUnit"`
	Rooms         []Room  `json:"rooms"`
}

func planCharts(raw string) ([]EvidenceChart, error) {
	if len(raw) > 20000 {
		return nil, errors.New("户型数据过大")
	}
	var p Plan
	if json.Unmarshal([]byte(raw), &p) != nil || !p.Confirmed || p.MetersPerUnit <= 0 || p.MetersPerUnit > 100 || len(p.Rooms) < 1 || len(p.Rooms) > 20 {
		return nil, errors.New("请确认户型、比例尺及房间轮廓")
	}
	rows := [][]string{}
	for _, r := range p.Rooms {
		if r.Name == "" || len([]rune(r.Name)) > 40 || len(r.Points) < 3 || len(r.Points) > 50 {
			return nil, errors.New("房间轮廓无效")
		}
		area, perimeter := 0., 0.
		for i, a := range r.Points {
			if a.X < 0 || a.Y < 0 || a.X > 10000 || a.Y > 10000 {
				return nil, errors.New("坐标超出范围")
			}
			b := r.Points[(i+1)%len(r.Points)]
			if a == b {
				return nil, errors.New("轮廓包含重复顶点")
			}
			area += a.X*b.Y - b.X*a.Y
			perimeter += math.Hypot(a.X-b.X, a.Y-b.Y)
			for j := i + 1; j < len(r.Points); j++ {
				if j == i+1 || i == 0 && j == len(r.Points)-1 {
					continue
				}
				if crosses(a, b, r.Points[j], r.Points[(j+1)%len(r.Points)]) {
					return nil, errors.New("房间轮廓自相交，请重新标注")
				}
			}
		}
		area = math.Abs(area) * p.MetersPerUnit * p.MetersPerUnit / 2
		if area <= 0 {
			return nil, errors.New("房间面积为零")
		}
		rows = append(rows, []string{r.Name, fmt.Sprintf("%.2f", area), fmt.Sprintf("%.2f", perimeter*p.MetersPerUnit)})
	}
	return []EvidenceChart{{Kind: "floorplan", Title: "已确认的户型轮廓", Note: "按用户校准的比例尺计算；图纸估算不是现场专业测绘。", Geometry: &p}, {Kind: "table", Title: "空间尺寸", Note: "独立房间面积，不对重叠房间求和。", Columns: []string{"房间", "面积 m²", "周长 m"}, Rows: rows}}, nil
}
func crosses(a, b, c, d Point) bool {
	cross := func(p, q, r Point) float64 { return (q.X-p.X)*(r.Y-p.Y) - (q.Y-p.Y)*(r.X-p.X) }
	x, y, z, w := cross(a, b, c), cross(a, b, d), cross(c, d, a), cross(c, d, b)
	return x*y <= 0 && z*w <= 0 && math.Max(a.X, b.X) >= math.Min(c.X, d.X) && math.Max(c.X, d.X) >= math.Min(a.X, b.X) && math.Max(a.Y, b.Y) >= math.Min(c.Y, d.Y) && math.Max(c.Y, d.Y) >= math.Min(a.Y, b.Y)
}

var mountainNames = []string{"子", "癸", "丑", "艮", "寅", "甲", "卯", "乙", "辰", "巽", "巳", "丙", "午", "丁", "未", "坤", "申", "庚", "酉", "辛", "戌", "乾", "亥", "壬"}
var mountainPalaces = []int{1, 1, 8, 8, 8, 3, 3, 3, 4, 4, 4, 9, 9, 9, 2, 2, 2, 7, 7, 7, 6, 6, 6, 1}

// Same-element dragon order in each palace: earth, heaven, human.
var palaceMountains = map[int][]int{1: {23, 0, 1}, 8: {2, 3, 4}, 3: {5, 6, 7}, 4: {8, 9, 10}, 9: {11, 12, 13}, 2: {14, 15, 16}, 7: {17, 18, 19}, 6: {20, 21, 22}}
var yangMountains = map[int]bool{23: true, 3: true, 4: true, 5: true, 9: true, 10: true, 11: true, 15: true, 16: true, 17: true, 21: true, 22: true}
var flight = []int{5, 6, 7, 8, 9, 1, 2, 3, 4}

func fly(center, sign int) map[int]int {
	m := map[int]int{}
	for i, p := range flight {
		m[p] = ((center-1+sign*i)%9+9)%9 + 1
	}
	return m
}
func flyingCharts(period int, facing float64) ([]EvidenceChart, error) {
	if period < 1 || period > 9 || math.IsNaN(facing) || math.IsInf(facing, 0) || facing < 0 || facing >= 360 {
		return nil, errors.New("请填写已确认的元运 1–9 和朝向 0–359.99°")
	}
	idx := int(math.Floor((facing+7.5)/15)) % 24
	offset := math.Abs(math.Mod(facing-float64(idx)*15+540, 360) - 180)
	if offset >= 4.5 {
		return nil, errors.New("当前算法仅支持下卦正向；兼向或分界请专业人员复核，不自动排盘")
	}
	sitting := (idx + 12) % 24
	base := fly(period, 1)
	sign := func(mountain, star int) int {
		original := mountain
		dragon := 0
		for i, m := range palaceMountains[mountainPalaces[mountain]] {
			if m == mountain {
				dragon = i
			}
		}
		if star != 5 {
			mountain = palaceMountains[star][dragon]
		} else {
			mountain = original
		}
		if yangMountains[mountain] {
			return 1
		}
		return -1
	}
	ms, ws := base[mountainPalaces[sitting]], base[mountainPalaces[idx]]
	mountain, water := fly(ms, sign(sitting, ms)), fly(ws, sign(idx, ws))
	items := []EvidenceItem{}
	labels := map[int]string{4: "巽 · 东南", 9: "离 · 南", 2: "坤 · 西南", 3: "震 · 东", 5: "中宫", 7: "兑 · 西", 8: "艮 · 东北", 1: "坎 · 北", 6: "乾 · 西北"}
	for _, p := range []int{4, 9, 2, 3, 5, 7, 8, 1, 6} {
		items = append(items, EvidenceItem{labels[p], fmt.Sprintf("%d / %d", mountain[p], water[p]), fmt.Sprintf("山星 / 向星 · 运星 %d", base[p])})
	}
	return []EvidenceChart{{Kind: "ninepalaces", Title: fmt.Sprintf("玄空下卦 · %d 运 · %s山%s向", period, mountainNames[sitting], mountainNames[idx]), Note: "南上北下；下卦三元龙阴阳顺逆飞，五黄按原山阴阳。元运由用户确认；不含替星、流年叠盘。算法 v1，传统文化参考。", Items: items}}, nil
}
func spatialCharts(in map[string]any) ([]EvidenceChart, error) {
	charts := []EvidenceChart{}
	if raw := stringValue(in["floorPlan"]); raw != "" {
		c, e := planCharts(raw)
		if e != nil {
			return nil, e
		}
		charts = append(charts, c...)
	}
	if stringValue(in["spatialMode"]) == "flying" {
		period, e := strconv.Atoi(stringValue(in["period"]))
		if e != nil {
			return nil, errors.New("元运无效")
		}
		bearing, e := strconv.ParseFloat(stringValue(in["facingDegrees"]), 64)
		if e != nil {
			return nil, e
		}
		c, e := flyingCharts(period, bearing)
		if e != nil {
			return nil, e
		}
		charts = append(charts, c...)
	}
	return charts, nil
}
