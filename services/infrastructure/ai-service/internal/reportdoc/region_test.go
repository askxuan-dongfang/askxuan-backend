package reportdoc

import "testing"

func TestReportShowsRepresentativePlaceWithItsTimeCalculation(t *testing.T) {
	d := &Document{}
	d.Add("bazi", `{"时间校正":{"输入时间":"1992-05-18T00:30","实际排盘时间":"1992-05-17T22:24"},"placeResolutionInfo":{"resolved":true,"representative":true,"formattedAddress":"中国新疆乌鲁木齐市沙依巴克区","level":"区县","usedLongitude":87.6,"usedLatitude":43.8,"coordinateReference":"WGS84","attribution":"GeoNames · CC BY 4.0","dataVersion":"2026-10-26"}}`)
	if len(d.Blocks) != 2 || d.Blocks[0].Title != "排盘时间核对" || d.Blocks[1].Title != "采用的地区坐标" {
		t.Fatalf("missing time/place evidence: %+v", d.Blocks)
	}
	if d.Blocks[1].Items[0].Value != "中国新疆乌鲁木齐市沙依巴克区" || d.Blocks[1].Note == "" {
		t.Fatalf("place provenance missing: %+v", d.Blocks[1])
	}
}
