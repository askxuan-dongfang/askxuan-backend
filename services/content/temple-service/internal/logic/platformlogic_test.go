package logic

import (
	"reflect"
	"testing"

	"github.com/askxuan/temple-service/internal/model"
)

func TestNormalizeTemplePlatformStatus(t *testing.T) {
	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		{input: "normal", want: model.TempleStatusNormal, ok: true},
		{input: model.TempleStatusNormal, want: model.TempleStatusNormal, ok: true},
		{input: "banned", want: model.TempleStatusBanned, ok: true},
		{input: "recommended", want: model.TempleStatusRecommend, ok: true},
		{input: "pending", ok: false},
	}
	for _, tt := range tests {
		got, ok := normalizeTemplePlatformStatus(tt.input)
		if got != tt.want || ok != tt.ok {
			t.Fatalf("normalizeTemplePlatformStatus(%q) = (%q, %v), want (%q, %v)", tt.input, got, ok, tt.want, tt.ok)
		}
	}
}

func TestWithTempleServiceSummaryOnlyCountsOnShelfServices(t *testing.T) {
	got := withTempleServiceSummary(toTypeTemple(&model.Temple{Code: "T001"}), []*model.TempleServiceRecord{
		{ServiceCode: "S001", ServiceName: "祈福", Status: model.TempleServiceStatusOnShelf},
		{ServiceCode: "S001", ServiceName: "祈福", Status: model.TempleServiceStatusOnShelf},
		{ServiceCode: "S002", ServiceName: "供灯", Status: model.TempleServiceStatusOffShelf},
		{ServiceCode: "S003", ServiceName: "上香", Status: model.TempleServiceStatusOnShelf},
	})
	if got.ServiceCount != 2 {
		t.Fatalf("ServiceCount = %d, want 2", got.ServiceCount)
	}
	if !reflect.DeepEqual(got.ServiceCodes, []string{"S001", "S003"}) {
		t.Fatalf("ServiceCodes = %#v", got.ServiceCodes)
	}
}
