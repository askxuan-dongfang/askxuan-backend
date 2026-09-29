package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/mozillazg/go-pinyin"
)

// Platform tools use fixed reviewed rules, never model-supplied rules or URLs.
type TopicEvidence struct {
	Charts       []EvidenceChart  `json:"charts,omitempty"`
	Version      string           `json:"version"`
	Source       string           `json:"source"`
	Title        string           `json:"title"`
	Note         string           `json:"note"`
	Columns      []string         `json:"columns"`
	Rows         [][]string       `json:"rows"`
	Calculations []map[string]any `json:"calculations,omitempty"`
}

func IsTopicTool(code string) bool {
	return oneOf(code, "naming", "fengshui", "dream", "date_select", "fortune")
}
func topicArguments(code, question string, inputs map[string]any) (string, error) {
	for _, d := range ReviewedTools() {
		if d.Code != code {
			continue
		}
		schema, _ := json.Marshal(d.InputSchema)
		raw, err := NewGuard(20000, nil).Validate(string(schema), question, inputs)
		if err != nil {
			return "", err
		}
		switch code {
		case "naming":
			if _, err := nameCandidates(inputs); err != nil {
				return "", err
			}
		case "date_select":
			if _, err := comparisonDates(inputs); err != nil {
				return "", err
			}
		case "fortune":
			if _, _, _, err := ParseBirthDate(stringValue(inputs["targetDate"]), "solar"); err != nil {
				return "", err
			}
			if stringValue(inputs["fortuneMode"]) == "personal" {
				b, _ := json.Marshal(birthFacts(inputs))
				if args, e := BuildToolArguments("bazi", question, string(b), time.Time{}); e != nil || args == "" {
					return "", errors.New("confirmed birth profile required")
				}
			}
		}
		return raw, nil
	}
	return "", errors.New("unknown topic tool")
}
func birthFacts(in map[string]any) map[string]any {
	out := map[string]any{}
	for _, k := range []string{"calendarType", "birthDate", "birthTime", "gender"} {
		if v, ok := in[k]; ok {
			out[k] = v
		}
	}
	return out
}
func nameCandidates(in map[string]any) ([]string, error) {
	surname := strings.TrimSpace(stringValue(in["surname"]))
	if utf8.RuneCountInString(surname) < 1 || utf8.RuneCountInString(surname) > 16 {
		return nil, errors.New("surname length must be 1–16")
	}
	raw := strings.TrimSpace(stringValue(in["name"]))
	if raw == "" && stringValue(in["mode"]) == "create" {
		return []string{surname}, nil
	}
	names := strings.FieldsFunc(raw, func(r rune) bool { return strings.ContainsRune(",，、;；\n", r) })
	if len(names) == 0 || len(names) > 8 {
		return nil, errors.New("provide 1–8 names")
	}
	for i, n := range names {
		n = strings.TrimSpace(n)
		if utf8.RuneCountInString(n) < 1 || utf8.RuneCountInString(n) > 16 {
			return nil, errors.New("name length must be 1–16")
		}
		names[i] = n
	}
	return names, nil
}
func namingEvidence(in map[string]any) (TopicEvidence, error) {
	names, err := nameCandidates(in)
	if err != nil {
		return TopicEvidence{}, err
	}
	out := TopicEvidence{Version: "20260929.1", Source: "go-pinyin@v0.21.0 / Unicode", Title: "名字读音与用字核验", Note: "逐字词典读音候选，多音字与姓氏读法须本人确认；不推断生僻程度，附字典笔画和适用范围内的五格；不生成吉凶分数。起名模式仅核验已填姓氏/候选，AI 新建议尚未核验。", Columns: []string{"已填名称", "字符数", "逐字读音候选", "待核对"}, Rows: [][]string{}}
	out.Charts = nameCharts(in, names)
	opts := pinyin.NewArgs()
	opts.Style = pinyin.Tone
	opts.Heteronym = true
	for _, name := range names {
		readings, notes := []string{}, []string{}
		seen := map[rune]bool{}
		for _, r := range name {
			if seen[r] {
				notes = append(notes, "重复字："+string(r))
			}
			seen[r] = true
			if strings.ContainsRune(stringValue(in["avoidChars"]), r) {
				notes = append(notes, "命中避用字："+string(r))
			}
			if !unicode.Is(unicode.Han, r) {
				readings = append(readings, string(r)+"：非汉字，保留原文")
				continue
			}
			ps := pinyin.SinglePinyin(r, opts)
			if len(ps) == 0 {
				readings = append(readings, string(r)+"：词典未覆盖")
			} else {
				readings = append(readings, string(r)+"："+strings.Join(ps, "/"))
				if len(ps) > 1 {
					notes = append(notes, string(r)+"为多音字")
				}
			}

		}
		if !strings.HasPrefix(name, stringValue(in["surname"])) {
			notes = append(notes, "与已填姓氏/主体前缀不一致")
		}
		if len(notes) == 0 {
			notes = append(notes, "仍需核对方言谐音及实际使用环境")
		}
		out.Rows = append(out.Rows, []string{name, fmt.Sprint(utf8.RuneCountInString(name)), strings.Join(readings, "；"), strings.Join(notes, "；")})
	}
	return out, nil
}
func spaceEvidence(in map[string]any) TopicEvidence {
	out := TopicEvidence{Version: "20260929.1", Source: "AskXuan 空间观察编辑规则 SPACE-1", Title: "空间观察与调整清单", Note: "本清单依据用户观察生成，不是建筑、消防验收。确认并校准的户型尺寸及所选下卦飞星另列图表，不推断住宅吉凶。", Columns: []string{"规则编号", "观察维度", "用户已确认", "下一步"}, Rows: [][]string{}}
	for _, r := range []struct {
		key, id, label string
		values         map[string][2]string
	}{
		{"daylight", "SPACE-LIGHT", "自然采光", map[string][2]string{"adequate": {"日常使用足够", "记录不同时间的光线变化"}, "dim": {"感觉偏暗", "检查遮挡与使用位置，比较调整前后照明"}, "glare": {"明显眩光", "调整屏幕与光源夹角，比较遮光前后效果"}}},
		{"ventilation", "SPACE-AIR", "通风", map[string][2]string{"openable": {"可开窗通风", "结合室外环境观察通风前后体感"}, "stuffy": {"感觉闷或气味滞留", "记录发生时段与可能来源，必要时请专业人员检查"}}},
		{"circulation", "SPACE-PATH", "主要动线", map[string][2]string{"clear": {"行走顺畅", "按实际起居路径复核"}, "blocked": {"占道或绕行", "移开可移动杂物，保持通道可通行；不自行改动承重结构"}}},
		{"noise", "SPACE-NOISE", "噪声", map[string][2]string{"quiet": {"不影响使用", "记录主要使用时段"}, "disturbing": {"影响休息或专注", "记录来源和时段，先比较家具位置或作息调整"}}},
	} {
		observed, ok := r.values[stringValue(in[r.key])]
		if !ok {
			observed = [2]string{"未确认", "先观察并记录，不推断房屋存在问题"}
		}
		out.Rows = append(out.Rows, []string{r.id, r.label, observed[0], observed[1]})
	}
	return out
}
func dreamEvidence(in map[string]any) TopicEvidence {
	out := TopicEvidence{Version: "20260929.1", Source: "AskXuan 梦境记录编辑规则 DREAM-1", Title: "梦境原述与反思线索", Note: "原述按用户填写保留，问题为编辑整理，不是诊断或古籍引文；不自动赋予象征含义，不预告现实事件。", Columns: []string{"规则编号", "线索", "用户原述", "反思问题"}, Rows: [][]string{}}
	for _, r := range [][4]string{{"DREAM-STORY", "梦境内容", "dream", "哪些场景是你最清楚记得的？"}, {"DREAM-FEELING", "醒来感受", "feeling", "这种感受与你近期的体验有没有联系？"}, {"DREAM-CONTEXT", "现实背景", "recentContext", "哪些是已发生的事实，哪些是自己的联想？"}} {
		v := stringValue(in[r[2]])
		if v == "" {
			v = "未填写"
		}
		out.Rows = append(out.Rows, []string{r[0], r[1], v, r[3]})
	}
	frequency := map[string]string{"unknown": "不确定", "once": "首次记录", "repeated": "曾重复出现"}[stringValue(in["recurrence"])]
	if frequency == "" {
		frequency = "未填写"
	}
	out.Rows = append(out.Rows, []string{"DREAM-RECURRENCE", "重复情况", frequency, "若再次出现，可以记录相同与变化之处。"})
	return out
}
func comparisonDates(in map[string]any) ([]string, error) {
	first := stringValue(in["targetDate"])
	if _, _, _, e := ParseBirthDate(first, "solar"); e != nil {
		return nil, e
	}
	dates := []string{first}
	if stringValue(in["dateMode"]) == "range" {
		end := stringValue(in["endDate"])
		if _, _, _, e := ParseBirthDate(end, "solar"); e != nil {
			return nil, e
		}
		start, _ := time.Parse("2006-01-02", first)
		last, _ := time.Parse("2006-01-02", end)
		days := int(last.Sub(start).Hours() / 24)
		if days < 0 || days > 13 {
			return nil, errors.New("range must contain 1–14 days")
		}
		dates = nil
		for i := 0; i <= days; i++ {
			dates = append(dates, start.AddDate(0, 0, i).Format("2006-01-02"))
		}
	} else {
		for _, k := range []string{"secondDate", "thirdDate"} {
			if v := stringValue(in[k]); v != "" {
				if _, _, _, e := ParseBirthDate(v, "solar"); e != nil {
					return nil, e
				}
				dates = append(dates, v)
			}
		}
	}
	out := []string{}
	seen := map[string]bool{}
	for _, d := range dates {
		t, _ := time.Parse("2006-01-02", d)
		if stringValue(in["weekdayOnly"]) == "weekdays" && (t.Weekday() == time.Saturday || t.Weekday() == time.Sunday) {
			continue
		}
		if !seen[d] {
			out = append(out, d)
			seen[d] = true
		}
	}
	if len(out) == 0 {
		return nil, errors.New("no dates remain after weekday preference")
	}
	return out, nil
}
func structuredEvidence(raw string) (map[string]any, error) {
	var out map[string]any
	if json.Unmarshal([]byte(raw), &out) == nil && len(out) > 0 {
		return out, nil
	}
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if json.Unmarshal([]byte(lines[i]), &out) == nil && len(out) > 0 {
			return out, nil
		}
	}
	return nil, errors.New("structured calculation result required")
}
func object(v any) map[string]any { m, _ := v.(map[string]any); return m }
func stringList(v any) ([]string, error) {
	a, ok := v.([]any)
	if !ok {
		return nil, errors.New("expected almanac string array")
	}
	out := make([]string, 0, len(a))
	for _, v := range a {
		s, ok := v.(string)
		if !ok || strings.TrimSpace(s) == "" {
			return nil, errors.New("invalid almanac item")
		}
		out = append(out, s)
	}
	return out, nil
}
func listed(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
func (c *MCPClient) topicRemote(ctx context.Context, code string, args map[string]any) (map[string]any, error) {
	cfg, _ := json.Marshal(ToolConfig{Enabled: true, Server: "taibu", Tool: code})
	b, _ := json.Marshal(args)
	raw, e := c.Call(ctx, string(cfg), string(b))
	if e != nil {
		return nil, e
	}
	return structuredEvidence(raw)
}
func (c *MCPClient) datesEvidence(ctx context.Context, in map[string]any) (TopicEvidence, error) {
	dates, e := comparisonDates(in)
	if e != nil {
		return TopicEvidence{}, e
	}
	out := TopicEvidence{Version: "20260929.1", Source: "taibu/almanac + AskXuan DATE-1 精确事项匹配", Title: "日期逐日对照", Note: "只匹配黄历实际宜忌，不作吉日评分；仅星期一至五不包含法定节假日或调休判断。任何日期查询失败则整次不发布，避免以缺失结果推荐。", Columns: []string{"日期", "星期", "事项匹配", "宜", "忌"}, Rows: [][]string{}}
	event := strings.TrimSpace(stringValue(in["event"]))
	if alias := map[string]string{"结婚": "嫁娶", "婚礼": "嫁娶", "开业": "开市", "搬家": "移徙", "入住": "入宅"}[event]; alias != "" {
		event = alias
	}
	for _, date := range dates {
		if e = ctx.Err(); e != nil {
			return out, e
		}
		m, e := c.topicRemote(ctx, "almanac", map[string]any{"date": date})
		if e != nil {
			return out, e
		}
		if stringValue(object(m["基础与个性化坐标"])["日期"]) != date {
			return out, errors.New("almanac date mismatch")
		}
		terms := object(m["择日宜忌"])
		if terms == nil || terms["宜"] == nil || terms["忌"] == nil {
			return out, errors.New("missing almanac suitable/avoid fields")
		}
		yes, err := stringList(terms["宜"])
		if err != nil {
			return out, err
		}
		no, err := stringList(terms["忌"])
		if err != nil {
			return out, err
		}
		status := "未列出（" + event + "）"
		if listed(yes, event) && listed(no, event) {
			status = "宜忌同时列出，需核对"
		} else if listed(no, event) {
			status = "忌：" + event
		} else if listed(yes, event) {
			status = "宜：" + event
		}
		t, _ := time.Parse("2006-01-02", date)
		day := []string{"日", "一", "二", "三", "四", "五", "六"}[t.Weekday()]
		out.Rows = append(out.Rows, []string{date, "星期" + day, status, strings.Join(yes, "、"), strings.Join(no, "、")})
		out.Calculations = append(out.Calculations, map[string]any{"tool": "almanac", "arguments": map[string]any{"date": date}, "data": m})
	}
	return out, nil
}
func (c *MCPClient) fortuneEvidence(ctx context.Context, in map[string]any) (TopicEvidence, error) {
	out := TopicEvidence{Version: "20260929.1", Source: "taibu/almanac", Title: "黄历与流日依据", Note: "个人模式只结合本人日主与流日，不等于完整命盘旺衰或大运判断；不生成个人吉凶分数。", Columns: []string{"依据", "已计算结果"}, Rows: [][]string{}}
	args := map[string]any{"date": stringValue(in["targetDate"])}
	if stringValue(in["fortuneMode"]) == "personal" {
		out.Source = "taibu/bazi + taibu/almanac"
		b, _ := json.Marshal(birthFacts(in))
		raw, e := BuildToolArguments("bazi", "", string(b), time.Time{})
		if e != nil {
			return out, e
		}
		var a map[string]any
		_ = json.Unmarshal([]byte(raw), &a)
		m, e := c.topicRemote(ctx, "bazi", a)
		if e != nil {
			return out, e
		}
		master := stringValue(object(m["基本信息"])["日主"])
		if !oneOf(master, "甲", "乙", "丙", "丁", "戊", "己", "庚", "辛", "壬", "癸") {
			return out, errors.New("calculation did not return day master")
		}
		args["dayMaster"] = master
		out.Rows = append(out.Rows, []string{"本人日主", master})
		out.Calculations = append(out.Calculations, map[string]any{"tool": "bazi", "arguments": a, "data": m})
	}
	m, e := c.topicRemote(ctx, "almanac", args)
	if e != nil {
		return out, e
	}
	coords := object(m["基础与个性化坐标"])
	if coords == nil || stringValue(coords["日期"]) != stringValue(in["targetDate"]) || stringValue(coords["日干支"]) == "" {
		return out, errors.New("missing day pillar")
	}
	out.Rows = append(out.Rows, []string{"查询日期", stringValue(in["targetDate"])}, []string{"流日干支", stringValue(coords["日干支"])})
	if args["dayMaster"] != nil {
		v := stringValue(coords["流日十神"])
		if strings.TrimSpace(v) == "" {
			return out, errors.New("missing personal day evidence")
		}
		b, _ := json.Marshal(v)
		out.Rows = append(out.Rows, []string{"流日十神", string(b)})
	}
	out.Calculations = append(out.Calculations, map[string]any{"tool": "almanac", "arguments": args, "data": m})
	return out, nil
}
func (c *MCPClient) callTopic(ctx context.Context, code, args string) (string, error) {
	if e := ctx.Err(); e != nil {
		return "", e
	}
	var in map[string]any
	if len(args) > 40000 || json.Unmarshal([]byte(args), &in) != nil {
		return "", errors.New("invalid topic inputs")
	}
	if _, e := topicArguments(code, "", in); e != nil {
		return "", e
	}
	var out TopicEvidence
	var err error
	switch code {
	case "naming":
		out, err = namingEvidence(in)
		if err == nil {
			out.Charts = append(out.Charts, nameSearch(ctx, in)...)
		}
	case "fengshui":
		out = spaceEvidence(in)
		out.Charts, err = spatialCharts(in)
	case "dream":
		out = dreamEvidence(in)
	case "date_select":
		out, err = c.datesEvidence(ctx, in)
	case "fortune":
		out, err = c.fortuneEvidence(ctx, in)
	default:
		return "", errors.New("unknown builtin tool")
	}
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(out)
	return string(b), err
}
