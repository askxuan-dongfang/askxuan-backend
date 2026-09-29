package agent

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
)

//go:embed data/kangxi-chars.json.gz
var kangxiData []byte
var dictionary struct {
	Chars map[string]struct {
		KX  int    `json:"kx"`
		BS  int    `json:"bs"`
		PY  string `json:"py"`
		Rad string `json:"rad"`
		T   string `json:"t"`
	} `json:"chars"`
	Alias map[string]string `json:"alias"`
}
var dictOnce sync.Once

func strokes(r rune) (int, int, string) {
	dictOnce.Do(func() {
		z, e := gzip.NewReader(bytes.NewReader(kangxiData))
		if e != nil {
			panic(e)
		}
		defer z.Close()
		if e = json.NewDecoder(z).Decode(&dictionary); e != nil {
			panic(e)
		}
	})
	key := string(r)
	e, ok := dictionary.Chars[key]
	if !ok {
		e, ok = dictionary.Chars[dictionary.Alias[key]]
	}
	if !ok {
		return 0, 0, ""
	}
	return e.KX, e.BS, e.T
}
func nameCharts(in map[string]any, names []string) []EvidenceChart {
	charts := []EvidenceChart{}
	surname := []rune(stringValue(in["surname"]))
	for _, name := range names {
		rows := [][]string{}
		counts := []int{}
		valid := true
		for _, r := range name {
			k, b, t := strokes(r)
			if k <= 0 {
				valid = false
				rows = append(rows, []string{string(r), "未覆盖", "—", "—"})
			} else {
				rows = append(rows, []string{string(r), strconv.Itoa(k), strconv.Itoa(b), t})
			}
			counts = append(counts, k)
		}
		charts = append(charts, EvidenceChart{Kind: "table", Title: name + " · 字典笔画", Note: "shunshi-kangxi-core@fad0bdf · 康熙与现代笔画分列；异体字以实际选择的字为准。", Columns: []string{"字", "康熙笔画", "现代笔画", "繁体候选"}, Rows: rows})
		given := len(counts) - len(surname)
		if !valid || !strings.HasPrefix(name, string(surname)) || len(surname) < 1 || len(surname) > 2 || given < 1 || given > 2 {
			continue
		}
		total, sky, earth := 0, 0, 0
		for i, n := range counts {
			total += n
			if i < len(surname) {
				sky += n
			} else {
				earth += n
			}
		}
		if len(surname) == 1 {
			sky++
		}
		if given == 1 {
			earth++
		}
		person := counts[len(surname)-1] + counts[len(surname)]
		outside := total - person
		if len(surname) == 1 {
			outside++
		}
		if given == 1 {
			outside++
		}
		items := []EvidenceItem{}
		for _, v := range []struct {
			n string
			v int
			d string
		}{{"天格", sky, "姓氏笔画和；单姓加一"}, {"人格", person, "姓末字＋名首字"}, {"地格", earth, "名字笔画和；单名加一"}, {"外格", outside, "总格减人格，单姓和单名各加一"}, {"总格", total, "全名康熙笔画和"}} {
			items = append(items, EvidenceItem{v.n, fmt.Sprint(v.v), v.d})
		}
		charts = append(charts, EvidenceChart{Kind: "pairs", Title: name + " · 五格计算", Note: "适用一至二字姓、一至二字名。传统规则计算，不提供吉凶打分。", Items: items})
	}
	return charts
}
