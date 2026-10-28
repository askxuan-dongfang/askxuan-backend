package reportdoc

import (
	"strings"
	"testing"
)

func TestDayunAgeLabelsPreserveEngineValues(t *testing.T) {
	d := New()
	d.Add("bazi_dayun", `{"起运信息":{"起运年龄":7,"起运详情":"6年3月3天"},"大运列表":[{"起运年份":1998,"起运年龄":7,"干支":"丙午"},{"起运年份":2018,"起运年龄":27,"干支":"戊申","流年列表":[{"流年":2026,"年龄":35,"干支":"丙午"}]}]}`)
	if len(d.Blocks) != 2 || !strings.Contains(d.Blocks[0].Note, "首次起运") || !strings.Contains(d.Blocks[0].Note, "虚岁") || !strings.Contains(d.Blocks[0].Items[1].Detail, "27虚岁") {
		t.Fatal(d.Blocks)
	}
	if d.Blocks[1].Columns[1] != "年龄（虚岁）" || d.Blocks[1].Rows[0][1] != "35" {
		t.Fatal("age value or basis changed", d.Blocks[1])
	}
}
