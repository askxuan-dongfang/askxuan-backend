package model

import "testing"

func TestDirectoryTagValidation(t *testing.T) {
	valid := DirectoryTag{Code: "mazu", Group: "tradition", Name: " 妈祖 / 天后 ", Value: "mazu", Sort: 10, Status: "enabled"}
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
