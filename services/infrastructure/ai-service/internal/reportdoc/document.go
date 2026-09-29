// Package reportdoc renders only observed tool data. The model writes the prose,
// but cannot manufacture a chart, score, palace or calculation source.
package reportdoc

import (
	"encoding/json"
	"strings"
)

type Item struct {
	Label  string `json:"label"`
	Value  string `json:"value"`
	Detail string `json:"detail,omitempty"`
}
type Block struct {
	Geometry json.RawMessage `json:"geometry,omitempty"`
	Kind     string          `json:"kind"`
	Title    string          `json:"title"`
	Source   string          `json:"source"`
	Note     string          `json:"note,omitempty"`
	Items    []Item          `json:"items,omitempty"`
	Columns  []string        `json:"columns,omitempty"`
	Rows     [][]string      `json:"rows,omitempty"`
}
type Evidence struct {
	Tool string `json:"tool"`
	Text string `json:"text"`
}
type Document struct {
	Version    int        `json:"version"`
	Runtime    string     `json:"runtime"`
	ModelCalls int        `json:"modelCalls"`
	ToolCalls  int        `json:"toolCalls"`
	Blocks     []Block    `json:"blocks"`
	Evidence   []Evidence `json:"evidence,omitempty"`
}

func New() Document { return Document{Version: 1, Runtime: "harness", Blocks: []Block{}} }
func scalar(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case nil:
		return "—"
	case []any:
		a := []string{}
		for _, v := range x {
			a = append(a, scalar(v))
		}
		return strings.Join(a, " · ")
	case map[string]any:
		b, _ := json.Marshal(x)
		return string(b)
	default:
		b, _ := json.Marshal(x)
		return string(b)
	}
}
func obj(v any) map[string]any { m, _ := v.(map[string]any); return m }
func list(v any) []any         { a, _ := v.([]any); return a }

// MCPClient appends one JSON line after the human-readable evidence.
func Structured(raw string) map[string]any {
	var full map[string]any
	if json.Unmarshal([]byte(raw), &full) == nil && len(full) > 0 {
		return full
	}
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		var m map[string]any
		if json.Unmarshal([]byte(lines[i]), &m) == nil && len(m) > 0 {
			return m
		}
	}
	return nil
}
func (d *Document) Add(tool, raw string) {
	if tool == "recall_memory" {
		return
	}
	for _, e := range d.Evidence {
		if e.Tool == tool && e.Text == raw {
			return
		}
	}
	d.Evidence = append(d.Evidence, Evidence{tool, raw})
	d.ToolCalls++
	m := Structured(raw)
	if m == nil {
		return
	}
	table := func(title string, fields []string, rows []any) {
		if len(rows) == 0 {
			return
		}
		b := Block{Kind: "table", Title: title, Source: tool, Columns: fields}
		for _, v := range rows {
			r := obj(v)
			cells := []string{}
			for _, k := range fields {
				cells = append(cells, scalar(r[k]))
			}
			b.Rows = append(b.Rows, cells)
		}
		d.Blocks = append(d.Blocks, b)
	}
	switch tool {
	case "search_knowledge":
		b := Block{Kind: "citations", Title: "知识依据", Source: tool, Note: "检索到的资料原文；引用仅在对应版本有效。"}
		for _, v := range list(m["hits"]) {
			r := obj(v)
			b.Items = append(b.Items, Item{scalar(r["id"]), scalar(r["text"]), scalar(r["title"]) + " · " + scalar(r["source"]) + " · " + scalar(r["locator"]) + " · v" + scalar(r["revision"]) + " · SHA256 " + scalar(r["sha256"])})
		}
		if len(b.Items) > 0 {
			d.Blocks = append(d.Blocks, b)
		}
	case "recall_memory":
		// Private memory is usable by the owner but is not copied into a report artifact.
		return

	case "naming", "fengshui", "dream", "date_select", "fortune":
		if scalar(m["version"]) != "20260929.1" {
			return
		}
		for _, v := range list(m["charts"]) {
			raw, _ := json.Marshal(v)
			var b Block
			if json.Unmarshal(raw, &b) == nil {
				switch b.Kind {
				case "table", "pairs", "ninepalaces", "floorplan":
					b.Source = tool
					d.Blocks = append(d.Blocks, b)
				}
			}
		}
		columns := []string{}
		for _, v := range list(m["columns"]) {
			columns = append(columns, scalar(v))
		}
		rows := [][]string{}
		for _, r := range list(m["rows"]) {
			row := []string{}
			for _, v := range list(r) {
				row = append(row, scalar(v))
			}
			if len(row) == len(columns) {
				rows = append(rows, row)
			}
		}
		if len(columns) > 0 && len(rows) > 0 {
			d.Blocks = append(d.Blocks, Block{Kind: "table", Title: scalar(m["title"]), Source: tool, Note: scalar(m["source"]) + " · " + scalar(m["note"]), Columns: columns, Rows: rows})
		}
		for _, raw := range list(m["calculations"]) {
			c := obj(raw)
			name := scalar(c["tool"])
			if name != "bazi" && name != "almanac" {
				continue
			}
			if tool == "date_select" {
				continue
			}
			data, _ := json.Marshal(c["data"])
			child := New()
			child.Add(name, string(data))
			d.Blocks = append(d.Blocks, child.Blocks...)
		}

	case "bazi_pillars_resolve":
		table("四柱对应的出生时间候选", []string{"候选序号", "公历", "农历", "出生时间", "是否闰月"}, list(m["候选列表"]))
	case "ziwei_horoscope":
		b := Block{Kind: "timeline", Title: "紫微运限叠宫", Source: tool, Note: "运限与本命宫位对应，不等同于事件预测。"}
		for _, v := range list(m["运限叠宫"]) {
			r := obj(v)
			b.Items = append(b.Items, Item{scalar(r["层次"]), scalar(r["落入本命宫位"]), scalar(r["时间段备注"]) + " · " + scalar(r["干支"]) + " · " + scalar(r["运限四化"])})
		}
		if len(b.Items) > 0 {
			d.Blocks = append(d.Blocks, b)
		}
		table("十二宫重排依据", []string{"层次", "十二宫重排", "运限四化"}, list(m["运限叠宫"]))
	case "ziwei_flying_star":
		for _, v := range list(m["查询结果"]) {
			r := obj(v)
			b := Block{Kind: "connections", Title: "紫微四化关系", Source: tool}
			for _, key := range []string{"实际飞化", "四化落宫"} {
				for _, v := range list(r[key]) {
					o := obj(v)
					b.Items = append(b.Items, Item{scalar(r["发射宫位"]) + " → " + scalar(o["宫位"]), scalar(o["四化"]), scalar(o["星曜"])})
				}
			}
			if len(b.Items) > 0 {
				d.Blocks = append(d.Blocks, b)
			}
		}
		table("四化查询", []string{"查询类型", "判断目标", "结果", "本宫"}, list(m["查询结果"]))
	case "astrology":
		b := Block{Kind: "zodiac", Title: "本命星体位置", Source: tool, Note: "只使用已计算的黄经、宫位和星座；坐标不足时不补造宫位。"}
		for _, v := range list(m["本命主星"]) {
			r := obj(v)
			b.Items = append(b.Items, Item{scalar(r["因素"]), scalar(r["星座"]), scalar(r["黄经"]) + " · " + scalar(r["宫位"]) + " · " + scalar(r["逆行"])})
		}
		if len(b.Items) > 0 {
			d.Blocks = append(d.Blocks, b)
		}
		table("本命相位", []string{"起点", "终点", "相位", "容许度", "实际夹角"}, list(obj(obj(m["扩展信息"])["完整相位矩阵"])["本命"]))
		table("宫位宫头", []string{"宫位", "宫头星座", "宫头黄经"}, list(obj(m["扩展信息"])["宫位宫头"]))
		table("流运触发", []string{"流运星体", "星座", "黄经", "宫位", "触发相位"}, list(m["当前流运触发"]))
	case "meihua":
		b := Block{Kind: "pairs", Title: "梅花本卦、互卦与变卦", Source: tool}
		plate := obj(m["卦盘"])
		for _, k := range []string{"本卦", "互卦", "变卦"} {
			r := obj(plate[k])
			if len(r) > 0 {
				b.Items = append(b.Items, Item{k, scalar(r["卦名"]), "上卦 " + scalar(r["上卦"]) + " · 下卦 " + scalar(r["下卦"])})
			}
		}
		if len(b.Items) > 0 {
			b.Note = "动爻：" + scalar(plate["动爻"])
			d.Blocks = append(d.Blocks, b)
		}
		table("阶段推演", []string{"阶段", "落点", "关系", "表达式"}, list(m["阶段推演"]))
	case "daliuren":
		b := Block{Kind: "palaces", Title: "六壬天地盘", Source: tool}
		for _, v := range list(m["天地盘"]) {
			r := obj(v)
			b.Items = append(b.Items, Item{scalar(r["地盘"]), scalar(r["天盘"]), scalar(r["天将"]) + " · " + scalar(r["遁干"])})
		}
		if len(b.Items) > 0 {
			d.Blocks = append(d.Blocks, b)
		}
		table("四课", []string{"课别", "乘将", "上神", "下神"}, list(m["四课"]))
		table("三传", []string{"传序", "地支", "天将", "六亲", "遁干"}, list(m["三传"]))
	case "xiaoliuren":
		b := Block{Kind: "timeline", Title: "小六壬推演链", Source: tool}
		for _, k := range []string{"月上起", "日上落", "时上落"} {
			if v, ok := obj(m["推演链"])[k]; ok {
				b.Items = append(b.Items, Item{k, scalar(v), ""})
			}
		}
		if len(b.Items) > 0 {
			b.Note = "落宫：" + scalar(obj(m["结果"])["落宫"])
			d.Blocks = append(d.Blocks, b)
		}
	case "taiyi":
		table("太乙九星阵列", []string{"观测层级", "太乙名", "北斗名", "五行", "方位", "宫位"}, list(m["九星阵列"]))
	case "bazi":
		rows := list(m["四柱"])
		b := Block{Kind: "pillars", Title: "四柱命盘", Source: tool, Note: "干支、十神与纳音来自本次排盘。"}
		for _, v := range rows {
			r := obj(v)
			b.Items = append(b.Items, Item{scalar(r["柱"]), scalar(r["干支"]), scalar(r["天干十神"]) + " · " + scalar(r["纳音"])})
		}
		if len(b.Items) > 0 {
			d.Blocks = append(d.Blocks, b)
		}
		for _, v := range rows {
			r := obj(v)
			hidden := []string{}
			for _, h := range list(r["藏干"]) {
				o := obj(h)
				hidden = append(hidden, scalar(o["天干"])+" · "+scalar(o["十神"])+"（"+scalar(o["气性"])+"）")
			}
			r["藏干明细"] = strings.Join(hidden, " / ")
		}
		table("四柱详表", []string{"柱", "干支", "天干十神", "藏干明细", "地势", "纳音", "神煞"}, rows)
		// Counts are intentionally unweighted. Never label these strength or fortune scores.
		counts := map[string]int{"木": 0, "火": 0, "土": 0, "金": 0, "水": 0}
		elements := map[rune]string{'甲': "木", '乙': "木", '寅': "木", '卯': "木", '丙': "火", '丁': "火", '巳': "火", '午': "火", '戊': "土", '己': "土", '辰': "土", '戌': "土", '丑': "土", '未': "土", '庚': "金", '辛': "金", '申': "金", '酉': "金", '壬': "水", '癸': "水", '亥': "水", '子': "水"}
		n := 0
		for _, v := range rows {
			for _, r := range scalar(obj(v)["干支"]) {
				if e := elements[r]; e != "" {
					counts[e]++
					n++
				}
			}
		}
		if n == 8 {
			b = Block{Kind: "elements", Title: "八字表层五行", Source: tool, Note: "统计四柱八个字的本五行，各字等权；不含藏干权重，不代表旺衰、喜忌或运势评分。"}
			for _, e := range []string{"木", "火", "土", "金", "水"} {
				b.Items = append(b.Items, Item{e, scalar(counts[e]), "个"})
			}
			d.Blocks = append(d.Blocks, b)
		}
	case "bazi_dayun":
		rows := list(m["大运列表"])
		b := Block{Kind: "timeline", Title: "大运时间轴", Source: tool, Note: scalar(obj(m["起运信息"])["起运详情"])}
		for _, v := range rows {
			r := obj(v)
			b.Items = append(b.Items, Item{scalar(r["起运年份"]), scalar(r["干支"]), scalar(r["起运年龄"]) + "岁 · " + scalar(r["十神"])})
		}
		if len(b.Items) > 0 {
			d.Blocks = append(d.Blocks, b)
		}
		for _, v := range rows {
			r := obj(v)
			table(scalar(r["干支"])+"大运 · 流年", []string{"流年", "年龄", "干支", "十神", "太岁关系"}, list(r["流年列表"]))
		}
	case "ziwei":
		b := Block{Kind: "palaces", Title: "紫微十二宫", Source: tool, Note: "按工具返回的宫位列示；点击展开星曜资料。"}
		for _, v := range list(m["十二宫位"]) {
			r := obj(v)
			stars := []string{}
			for _, k := range []string{"主星及四化", "辅星"} {
				for _, s := range list(r[k]) {
					o := obj(s)
					txt := scalar(o["星名"])
					for _, f := range []string{"亮度", "四化", "离心自化", "向心自化"} {
						if o[f] != nil {
							txt += " · " + scalar(o[f])
						}
					}
					stars = append(stars, txt)
				}
			}
			if len(stars) == 0 {
				stars = []string{"无主星"}
			}
			b.Items = append(b.Items, Item{scalar(r["宫位"]) + " · " + scalar(r["干支"]), strings.Join(stars, " / "), "大限 " + scalar(r["大限"])})
		}
		if len(b.Items) > 0 {
			d.Blocks = append(d.Blocks, b)
		}
	case "liuyao":
		hex := Block{Kind: "pairs", Title: "本卦与变卦", Source: tool}
		for _, key := range []string{"本卦", "变卦"} {
			g := obj(obj(m["卦盘"])[key])
			if g["卦名"] != nil {
				hex.Items = append(hex.Items, Item{key, scalar(g["卦名"]), scalar(g["卦辞"])})
			}
		}
		if len(hex.Items) > 0 {
			d.Blocks = append(d.Blocks, hex)
		}
		rows := list(obj(m["六爻全盘"])["爻列表"])
		b := Block{Kind: "hexagram", Title: "六爻盘面", Source: tool}
		for _, v := range rows {
			r := obj(v)
			p := obj(r["本爻"])
			b.Items = append(b.Items, Item{scalar(r["爻位"]), scalar(r["六神"]) + " · " + scalar(p["六亲"]) + " · " + scalar(p["纳甲"]), scalar(r["动静"])})
		}
		if len(b.Items) > 0 {
			d.Blocks = append(d.Blocks, b)
		}
	case "qimen":
		grid := Block{Kind: "palaces", Title: "奇门九宫", Source: tool, Note: "南在上、北在下；点击宫位展开本次工具返回的星、门、神资料。"}
		for _, n := range []string{"4", "9", "2", "3", "5", "7", "8", "1", "6"} {
			for _, v := range list(m["九宫盘"]) {
				r := obj(v)
				if scalar(r["宫位序号"]) != n {
					continue
				}
				grid.Items = append(grid.Items, Item{scalar(r["宫名"]) + " · " + scalar(r["方位"]), scalar(r["九星"]) + " / " + scalar(r["八门"]) + " / " + scalar(r["八神"]), "天盘 " + scalar(r["天盘天干"]) + " · 地盘 " + scalar(r["地盘天干"])})
			}
		}
		if len(grid.Items) > 0 {
			d.Blocks = append(d.Blocks, grid)
		}
		table("奇门九宫明细", []string{"方位", "宫名", "九星", "八门", "八神", "天盘天干", "地盘天干"}, list(m["九宫盘"]))
	case "tarot":
		b := Block{Kind: "cards", Title: "本次牌阵", Source: tool, Note: "牌面来自本次抽牌结果，刷新报告不会重新抽牌。"}
		for _, v := range list(m["牌阵展开"]) {
			r := obj(v)
			b.Items = append(b.Items, Item{scalar(r["位置"]), scalar(r["塔罗牌"]), scalar(r["状态"]) + " · " + scalar(r["核心基调"])})
		}
		if len(b.Items) > 0 {
			d.Blocks = append(d.Blocks, b)
		}
	case "almanac":
		b := Block{Kind: "pairs", Title: "黄历 · " + scalar(obj(m["基础与个性化坐标"])["日期"]), Source: tool, Note: "传统历法信息，实际安排仍需结合档期与现实条件。"}
		for _, k := range []string{"宜", "忌"} {
			b.Items = append(b.Items, Item{k, scalar(obj(m["择日宜忌"])[k]), ""})
		}
		d.Blocks = append(d.Blocks, b)
	}
}
