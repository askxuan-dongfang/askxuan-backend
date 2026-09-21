// Package experience provides versioned, deterministic skills. No model is allowed
// to invent dictionary sources or replace the user's decision scores.
package experience

import (
	"fmt"
	"math"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"
)

const Version = "2026-09-21.1"

type NamingInput struct {
	Purpose string `json:"purpose"`
	Style   string `json:"style"`
	Surname string `json:"surname"`
	Avoid   string `json:"avoid"`
}
type Factor struct {
	Label  string `json:"label"`
	Weight int    `json:"weight"`
	A      int    `json:"a"`
	B      int    `json:"b"`
}
type DecisionInput struct {
	A           string   `json:"a"`
	B           string   `json:"b"`
	ConstraintA string   `json:"constraintA"`
	ConstraintB string   `json:"constraintB"`
	Factors     []Factor `json:"factors"`
}
type Input struct {
	Skill    string         `json:"skill"`
	Naming   *NamingInput   `json:"naming,omitempty"`
	Decision *DecisionInput `json:"decision,omitempty"`
}
type Character struct {
	Text    string `json:"text"`
	Reading string `json:"reading"`
	Meaning string `json:"meaning"`
	Source  string `json:"source"`
}
type Candidate struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Idea       string      `json:"idea"`
	Characters []Character `json:"characters"`
}
type Contribution struct {
	Factor
	ContributionA float64 `json:"contributionA"`
	ContributionB float64 `json:"contributionB"`
}
type Comparison struct {
	ScoreA    float64        `json:"scoreA"`
	ScoreB    float64        `json:"scoreB"`
	EligibleA bool           `json:"eligibleA"`
	EligibleB bool           `json:"eligibleB"`
	Factors   []Contribution `json:"factors"`
}
type Result struct {
	Skill      string      `json:"skill"`
	Version    string      `json:"version"`
	Basis      string      `json:"basis"`
	Summary    string      `json:"summary"`
	NextSteps  []string    `json:"nextSteps"`
	Candidates []Candidate `json:"candidates,omitempty"`
	Comparison *Comparison `json:"comparison,omitempty"`
}
type entry struct{ text, reading, meaning string }

// Common senses paraphrased from 汉典 basic definitions, checked 2026-09-21.
// The pair ideas below are editorial associations, never claimed as quotations.
var lexicon = []entry{
	{"安", "ān", "平静、安定"}, {"宁", "níng", "安宁、平静"}, {"清", "qīng", "清澈、清晰"}, {"和", "hé", "和谐、平和"},
	{"明", "míng", "明亮、清楚"}, {"远", "yuǎn", "距离长，与近相对"}, {"云", "yún", "天空中的云"}, {"舒", "shū", "舒展、从容"},
	{"林", "lín", "成片的树木"}, {"禾", "hé", "谷类植物的统称"}, {"初", "chū", "起始、开端"}, {"晴", "qíng", "雨止而无云或少云"},
	{"星", "xīng", "夜空中发光的星体"}, {"言", "yán", "说话、言语"}, {"知", "zhī", "知道、理解"}, {"行", "xíng", "行走、做"},
}

type pair struct{ name, style, idea string }

var pairs = []pair{
	{"安宁", "gentle", "安定而从容"}, {"清和", "gentle", "清朗与平和"}, {"舒宁", "gentle", "舒展与宁静"}, {"和初", "gentle", "以平和迎接新开始"},
	{"云舒", "nature", "云朵舒展的意象"}, {"星禾", "nature", "星光与禾苗的意象"}, {"林晴", "nature", "晴日林间的意象"}, {"初晴", "nature", "雨后放晴的意象"},
	{"知行", "clear", "理解与行动相伴"}, {"明远", "clear", "清晰看待长远"}, {"知言", "clear", "理解与表达"}, {"明初", "clear", "明朗的开始"},
}

func textOK(s string, min, max int) bool {
	n := utf8.RuneCountInString(strings.TrimSpace(s))
	return n >= min && n <= max
}
func han(s string) bool {
	for _, r := range s {
		if !unicode.Is(unicode.Han, r) {
			return false
		}
	}
	return true
}
func Run(in Input) (Result, error) {
	r := Result{Skill: in.Skill, Version: Version}
	switch in.Skill {
	case "naming":
		if in.Naming == nil || in.Decision != nil {
			return r, fmt.Errorf("请填写姓名灵感资料")
		}
		n := in.Naming
		if (n.Purpose != "name" && n.Purpose != "pen") || (n.Style != "gentle" && n.Style != "nature" && n.Style != "clear") || !textOK(n.Surname, 0, 2) || !han(n.Surname) || !textOK(n.Avoid, 0, 16) || !han(n.Avoid) || (n.Purpose == "pen" && n.Surname != "") {
			return r, fmt.Errorf("请检查用途、风格、姓氏和避用字")
		}
		r.Basis = "词库筛选 · 字义参考汉典；组合意象为编辑联想，不是古籍原句。读音为候选名字采用的常见读法。"
		r.Candidates = []Candidate{}
		for _, p := range pairs {
			if p.style != n.Style || strings.ContainsAny(p.name, n.Avoid+n.Surname) {
				continue
			}
			c := Candidate{ID: p.name, Name: n.Surname + p.name, Idea: p.idea, Characters: []Character{}}
			for _, ch := range p.name {
				for _, e := range lexicon {
					if e.text == string(ch) {
						c.Characters = append(c.Characters, Character{e.text, e.reading, e.meaning, "https://www.zdic.net/hans/" + url.PathEscape(e.text)})
					}
				}
			}
			r.Candidates = append(r.Candidates, c)
		}
		r.Summary = fmt.Sprintf("找到 %d 个符合当前风格与避用字的候选", len(r.Candidates))
		if len(r.Candidates) == 0 {
			r.Summary = "当前词库没有符合条件的候选，可调整风格或避用字。"
		}
		r.NextSteps = []string{"挑选两个喜欢的候选，比较读音和字义。", "连同姓氏多读几遍，检查方言谐音；正式使用前自行核对重名与登记要求。"}
	case "decision":
		if in.Decision == nil || in.Naming != nil {
			return r, fmt.Errorf("请填写两种选择")
		}
		d := in.Decision
		if !textOK(d.A, 1, 60) || !textOK(d.B, 1, 60) || strings.TrimSpace(d.A) == strings.TrimSpace(d.B) || !textOK(d.ConstraintA, 0, 200) || !textOK(d.ConstraintB, 0, 200) || len(d.Factors) < 2 || len(d.Factors) > 6 {
			return r, fmt.Errorf("请填写两个不同的选项和 2–6 项比较因素")
		}
		total, a, b := 0, 0, 0
		seen := map[string]bool{}
		for _, f := range d.Factors {
			label := strings.TrimSpace(f.Label)
			if !textOK(label, 1, 30) || seen[label] || f.Weight < 0 || f.Weight > 5 || f.A < 0 || f.A > 5 || f.B < 0 || f.B > 5 {
				return r, fmt.Errorf("因素名称不能重复；重要性、满足度须为 0–5 的整数")
			}
			seen[label] = true
			total += f.Weight
			a += f.Weight * f.A
			b += f.Weight * f.B
		}
		if total == 0 {
			return r, fmt.Errorf("至少为一个因素设置大于 0 的重要性")
		}
		score := func(x int) float64 { return math.Round(float64(x)/float64(total)*20*10) / 10 }
		c := &Comparison{ScoreA: score(a), ScoreB: score(b), EligibleA: strings.TrimSpace(d.ConstraintA) == "", EligibleB: strings.TrimSpace(d.ConstraintB) == "", Factors: []Contribution{}}
		for _, f := range d.Factors {
			c.Factors = append(c.Factors, Contribution{f, score(f.Weight * f.A), score(f.Weight * f.B)})
		}
		r.Comparison = c
		r.Basis = "你的主观评分 × 重要性，合计后 ÷（重要性总和 × 5）× 100。分数不是成功概率；硬约束优先于分数。"
		switch {
		case !c.EligibleA && !c.EligibleB:
			r.Summary = "两种选择都有未解决的硬约束，先补充条件或寻找第三种方案。"
		case !c.EligibleA:
			r.Summary = "A 有未解决的硬约束，暂不纳入选择；先核实 B 的可行性。"
		case !c.EligibleB:
			r.Summary = "B 有未解决的硬约束，暂不纳入选择；先核实 A 的可行性。"
		case a == b:
			r.Summary = "按当前权重，两种选择持平；尝试补充最重要的差异。"
		case a > b:
			r.Summary = "按你当前的评分与权重，A 更符合你看重的因素。"
		default:
			r.Summary = "按你当前的评分与权重，B 更符合你看重的因素。"
		}
		r.NextSteps = []string{"调整最重要因素的权重，观察结论是否改变。", "为不确定的评分补一条现实依据，再做一次比较。", "选择一个低成本、可撤回的小行动；重要决定仍需结合现实和专业意见。"}
	default:
		return r, fmt.Errorf("不支持的体验")
	}
	return r, nil
}
