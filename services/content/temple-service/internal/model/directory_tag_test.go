package model

import "testing"

func TestDirectoryTagValidation(t *testing.T) {
	valid := DirectoryTag{Code: "mazu", Group: "tradition", BeliefCode: "folk", Name: " 妈祖 / 天后 ", Value: "mazu", Sort: 10, Status: "enabled"}
	if err := ValidateDirectoryTag(&valid); err != nil || valid.Name != "妈祖 / 天后" {
		t.Fatalf("valid label: %+v %v", valid, err)
	}
	for _, change := range []func(*DirectoryTag){func(p *DirectoryTag) { p.Group = "sql" }, func(p *DirectoryTag) { p.Code = "../unsafe" }, func(p *DirectoryTag) { p.Value = "" }, func(p *DirectoryTag) { p.Value = "a,b" }, func(p *DirectoryTag) { p.Name = "" }, func(p *DirectoryTag) { p.Status = "hidden" }, func(p *DirectoryTag) { p.Sort = -1 }} {
		candidate := valid
		change(&candidate)
		if ValidateDirectoryTag(&candidate) == nil {
			t.Fatalf("invalid label accepted: %+v", candidate)
		}
	}
}

func TestDirectoryBeliefParentValidation(t *testing.T) {
	for _, code := range []string{"han_buddhism", "tibetan_buddhism", "theravada_buddhism", "daoism", "taoism", "folk"} {
		p := DirectoryTag{Code: "test_tag", Group: "tradition", BeliefCode: code, Name: "测试", Value: "test_tag", Status: "enabled"}
		if err := ValidateDirectoryTag(&p); err != nil {
			t.Fatalf("%s: %v", code, err)
		}
		if code == "taoism" && p.BeliefCode != "daoism" {
			t.Fatal("alias not normalized")
		}
	}
	for _, code := range []string{"", "buddhism", "unknown", "folk,daoism"} {
		p := DirectoryTag{Code: "test_tag", Group: "tradition", BeliefCode: code, Name: "测试", Value: "test_tag", Status: "enabled"}
		if ValidateDirectoryTag(&p) == nil {
			t.Fatalf("invalid parent accepted: %q", code)
		}
	}
	p := DirectoryTag{Code: "position_01", Group: "position", BeliefCode: "folk", Name: "讲师", Value: "讲师", Status: "enabled"}
	if ValidateDirectoryTag(&p) == nil {
		t.Fatal("position cannot have belief parent")
	}
}
