package experience

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNamingEvidenceAndConstraints(t *testing.T) {
	for _, style := range []string{"gentle", "nature", "clear"} {
		r, e := Run(Input{Skill: "naming", Naming: &NamingInput{Purpose: "name", Style: style, Surname: "王"}})
		if e != nil || len(r.Candidates) != 4 {
			t.Fatalf("%s: %v", style, e)
		}
		for _, c := range r.Candidates {
			if !strings.HasPrefix(c.Name, "王") || len(c.Characters) != 2 {
				t.Fatal(c)
			}
			for _, ch := range c.Characters {
				if ch.Meaning == "" || ch.Reading == "" || !strings.HasPrefix(ch.Source, "https://www.zdic.net/hans/") {
					t.Fatal("ungrounded character")
				}
			}
		}
	}
	r, e := Run(Input{Skill: "naming", Naming: &NamingInput{Purpose: "name", Style: "gentle", Surname: "清", Avoid: "宁初"}})
	if e != nil || len(r.Candidates) != 0 {
		t.Fatal("forbidden characters used", r, e)
	}
	for _, n := range []NamingInput{{Purpose: "oops", Style: "gentle"}, {Purpose: "name", Style: "unknown"}, {Purpose: "name", Style: "clear", Surname: "abc"}, {Purpose: "pen", Style: "clear", Surname: "林"}} {
		if _, e := Run(Input{Skill: "naming", Naming: &n}); e == nil {
			t.Fatal("invalid naming accepted", n)
		}
	}
}
func decision() Input {
	return Input{Skill: "decision", Decision: &DecisionInput{A: "郊外散步", B: "在家休息", Factors: []Factor{{"个人感受", 3, 5, 2}, {"节省时间", 1, 1, 5}}}}
}
func TestDecisionMathAndWeightSensitivity(t *testing.T) {
	in := decision()
	r, e := Run(in)
	if e != nil || r.Comparison.ScoreA != 80 || r.Comparison.ScoreB != 55 {
		t.Fatal(r, e)
	}
	if r.Comparison.Factors[0].ContributionA != 75 {
		t.Fatal("wrong factor contribution")
	}
	in.Decision.Factors[0].Weight = 1
	in.Decision.Factors[1].Weight = 5
	r, e = Run(in)
	if e != nil || r.Comparison.ScoreA >= r.Comparison.ScoreB {
		t.Fatal("weight adjustment must change ranking")
	}
	in.Decision.ConstraintB = "超过可支配预算"
	r, e = Run(in)
	if e != nil || r.Comparison.EligibleB || !strings.Contains(r.Summary, "硬约束") {
		t.Fatal("score overrode constraint")
	}
	in.Decision.ConstraintA = "时间不足"
	r, _ = Run(in)
	if !strings.Contains(r.Summary, "第三种") {
		t.Fatal("both blocked still recommended")
	}
}
func TestDecisionRejectsInvalidAndHandlesTies(t *testing.T) {
	for _, mutate := range []func(*DecisionInput){func(d *DecisionInput) { d.B = d.A }, func(d *DecisionInput) { d.Factors[1].Label = d.Factors[0].Label }, func(d *DecisionInput) { d.Factors[0].A = 6 }, func(d *DecisionInput) { d.Factors[0].Weight = -1 }, func(d *DecisionInput) {
		for i := range d.Factors {
			d.Factors[i].Weight = 0
		}
	}, func(d *DecisionInput) { d.Factors = d.Factors[:1] }} {
		in := decision()
		mutate(in.Decision)
		if _, e := Run(in); e == nil {
			t.Fatal("invalid decision accepted")
		}
	}
	in := decision()
	for i := range in.Decision.Factors {
		in.Decision.Factors[i].B = in.Decision.Factors[i].A
	}
	r, _ := Run(in)
	if !strings.Contains(r.Summary, "持平") {
		t.Fatal(r.Summary)
	}
	first, _ := json.Marshal(r)
	r, _ = Run(in)
	second, _ := json.Marshal(r)
	if string(first) != string(second) {
		t.Fatal("non-reproducible result")
	}
}
