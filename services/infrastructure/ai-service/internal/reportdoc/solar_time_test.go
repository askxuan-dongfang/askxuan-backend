package reportdoc

import "testing"

func TestTimeCorrectionAndDeduplicationKeepSubjects(t *testing.T) {
	d := New()
	d.Add("bazi", `{"时间校正":{"输入时间":"1992-05-18T00:30","实际排盘时间":"1992-05-17T22:24","时间口径":"真太阳时","经度":87.6168,"跨日偏移":-1}}`)
	if len(d.Blocks) != 1 || d.Blocks[0].Title != "排盘时间核对" || len(d.Blocks[0].Items) != 5 {
		t.Fatal(d.Blocks)
	}
	chart := Block{Kind: "pairs", Title: "黄历 · 2026-10-27", Source: "fortune", Items: []Item{{Label: "宜", Value: "祭祀"}}}
	d.Blocks = append(d.Blocks, chart)
	chart.Source = "almanac"
	d.Blocks = append(d.Blocks, chart)
	chart.Title = "你 · 四柱命盘"
	d.Blocks = append(d.Blocks, chart)
	chart.Title = "对方 · 四柱命盘"
	d.Blocks = append(d.Blocks, chart)
	d.DeduplicateBlocks()
	if len(d.Blocks) != 4 {
		t.Fatal("lost subjects or retained duplicate", d.Blocks)
	}
}
