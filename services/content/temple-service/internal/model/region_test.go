package model

import "testing"

func TestRegionValidation(t *testing.T) {
	for _, code := range []string{"mainland", "hong_kong", "macau", "taiwan", "southeast_asia", "europe"} {
		p := RegionTag{Code: code, Name: " 地区 ", Sort: 10, Status: "enabled"}
		if err := ValidateRegion(&p); err != nil || p.Name != "地区" {
			t.Fatalf("valid region rejected: %+v %v", p, err)
		}
	}
	for _, p := range []RegionTag{{Code: "../x", Name: "地区", Status: "enabled"}, {Code: "mainland", Name: " ", Status: "enabled"}, {Code: "mainland", Name: "地区", Sort: -1, Status: "enabled"}, {Code: "mainland", Name: "地区", Status: "other"}} {
		if ValidateRegion(&p) == nil {
			t.Fatalf("invalid region accepted: %+v", p)
		}
	}
}
func TestRegionFilterIsBoundBeforePagination(t *testing.T) {
	where, args := buildTempleWhere(TempleFilter{RegionCode: "hong_kong", Statuses: TemplePublicStatuses()})
	if len(args) < 1 || args[0] != "hong_kong" {
		t.Fatalf("missing region filter: %s %+v", where, args)
	}
}
