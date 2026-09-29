package agent

import (
	"context"
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestKangxiVariantsAndWuge(t *testing.T) {
	a, _, _ := strokes('于')
	b, _, _ := strokes('於')
	if a != 3 || b != 8 {
		t.Fatalf("collapsed variants %d %d", a, b)
	}
	charts := nameCharts(map[string]any{"surname": "李"}, []string{"李明"})
	if len(charts) != 2 {
		t.Fatal(charts)
	}
	want := []string{"8", "15", "9", "2", "15"}
	for i, item := range charts[1].Items {
		if item.Value != want[i] {
			t.Fatal(item)
		}
	}
	if len(nameCharts(map[string]any{"surname": "李"}, []string{"李🙂"})) != 1 {
		t.Fatal("unknown stroke was invented")
	}
}
func TestFloorScaleAndInvalidGeometry(t *testing.T) {
	raw := `{"confirmed":true,"metersPerUnit":0.01,"rooms":[{"name":"客厅","points":[{"x":0,"y":0},{"x":400,"y":0},{"x":400,"y":300},{"x":0,"y":300}]}]}`
	c, e := planCharts(raw)
	if e != nil || c[1].Rows[0][1] != "12.00" || c[1].Rows[0][2] != "14.00" {
		t.Fatalf("%+v %v", c, e)
	}
	for _, v := range []string{strings.Replace(raw, `"confirmed":true`, `"confirmed":false`, 1), strings.Replace(raw, `"metersPerUnit":0.01`, `"metersPerUnit":0`, 1), `{"confirmed":true,"metersPerUnit":1,"rooms":[{"name":"X","points":[{"x":0,"y":0},{"x":2,"y":2},{"x":0,"y":2},{"x":2,"y":0}]}]}`} {
		if _, e := planCharts(v); e == nil {
			t.Fatal("invalid geometry accepted")
		}
	}
}
func TestFlyingStarLoShuAndDirectionBounds(t *testing.T) {
	for period := 1; period <= 9; period++ {
		for i := 0; i < 24; i++ {
			c, e := flyingCharts(period, float64(i*15))
			if e != nil || len(c[0].Items) != 9 {
				t.Fatalf("%d/%d: %v", period, i, e)
			}
			if c[0].Items[4].Label != "中宫" {
				t.Fatal("grid orientation")
			}
		}
	}
	for _, angle := range []float64{4.5, 7.5, 355.5, -1, 360, math.NaN()} {
		if _, e := flyingCharts(9, angle); e == nil {
			t.Fatal("boundary accepted", angle)
		}
	}
	base := fly(9, 1)
	if base[5] != 9 || base[6] != 1 || base[4] != 8 {
		t.Fatal(base)
	}
	c, _ := flyingCharts(9, 180)
	if c[0].Items[4].Value != "5 / 4" {
		t.Fatal(c[0].Items[4])
	}
}
func TestExternalLookupUnconfiguredIsNotEmptySuccess(t *testing.T) {
	t.Setenv("AI_NAME_DUPLICATE_URL", "")
	out := nameSearch(context.Background(), map[string]any{"nameSearch": "duplicate", "surname": "李", "name": "李明"})
	b, _ := json.Marshal(out)
	if !strings.Contains(string(b), "未接通") {
		t.Fatal(string(b))
	}
}
