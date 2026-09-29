package reportdoc

import (
	"encoding/json"
	"testing"
)

func TestPrivateMemoryNotExported(t *testing.T) {
	d := New()
	d.Add("recall_memory", `{"hits":[{"text":"private"}]}`)
	if len(d.Evidence) != 0 || len(d.Blocks) != 0 {
		t.Fatal(d)
	}
}
func TestCitationsAndDomainCharts(t *testing.T) {
	d := New()
	d.Add("search_knowledge", `{"hits":[{"id":"a","title":"资料","text":"原文","source":"书名","locator":"第1页","revision":2,"sha256":"verified"}]}`)
	if len(d.Blocks) != 1 || d.Blocks[0].Items[0].Value != "原文" {
		t.Fatal(d)
	}
	d.Add("fengshui", `{"version":"20260929.1","charts":[{"kind":"ninepalaces","title":"盘面","items":[{"label":"中宫","value":"5 / 4"}]},{"kind":"script","title":"evil"}]}`)
	if len(d.Blocks) != 2 || d.Blocks[1].Kind != "ninepalaces" {
		t.Fatal(d)
	}
	_, e := json.Marshal(d)
	if e != nil {
		t.Fatal(e)
	}
}

func TestAdditionalToolChartSchemas(t *testing.T) {
	fixtures := map[string]string{
		"bazi_pillars_resolve": `{"候选列表":[{"候选序号":1,"公历":"2000-01-01","出生时间":"12:00"}]}`,
		"ziwei_horoscope":      `{"运限叠宫":[{"层次":"流年","落入本命宫位":"命宫","干支":"甲子"}]}`,
		"ziwei_flying_star":    `{"查询结果":[{"发射宫位":"命宫","实际飞化":[{"宫位":"财帛宫","四化":"禄","星曜":"太阳"}]}]}`,
		"astrology":            `{"本命主星":[{"因素":"太阳","星座":"白羊座","黄经":"15°","宫位":"第1宫"}]}`,
		"meihua":               `{"卦盘":{"本卦":{"卦名":"乾","上卦":"乾","下卦":"乾"},"动爻":1}}`,
		"daliuren":             `{"天地盘":[{"地盘":"子","天盘":"午","天将":"青龙"}]}`,
		"xiaoliuren":           `{"推演链":{"月上起":"大安","日上落":"留连","时上落":"速喜"},"结果":{"落宫":"速喜"}}`,
		"taiyi":                `{"九星阵列":[{"太乙名":"太乙","宫位":1,"五行":"水"}]}`,
	}
	for tool, raw := range fixtures {
		t.Run(tool, func(t *testing.T) {
			d := New()
			d.Add(tool, raw)
			if len(d.Blocks) == 0 {
				t.Fatal("missing chart")
			}
			for _, b := range d.Blocks {
				if b.Source != tool || len(b.Items)+len(b.Rows) == 0 {
					t.Fatalf("ungrounded block %+v", b)
				}
			}
			empty := New()
			empty.Add(tool, `{}`)
			if len(empty.Blocks) > 0 {
				t.Fatal("fabricated empty chart")
			}
		})
	}
}
