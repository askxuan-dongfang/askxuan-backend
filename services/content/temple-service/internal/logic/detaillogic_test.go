package logic

import (
	"testing"

	"github.com/askxuan/temple-service/internal/model"
)

func TestPublicTempleServiceRecordsOnlyReturnsOnShelf(t *testing.T) {
	services := []*model.TempleServiceRecord{
		{Id: 1, ServiceCode: "S001", Status: model.TempleServiceStatusOnShelf},
		nil,
		{Id: 2, ServiceCode: "S002", Status: "off_shelf"},
		{Id: 3, ServiceCode: "S003", Status: model.TempleServiceStatusOnShelf},
	}

	public := publicTempleServiceRecords(services)
	if len(public) != 2 {
		t.Fatalf("expected 2 public services, got %d", len(public))
	}
	if public[0].ServiceCode != "S001" || public[1].ServiceCode != "S003" {
		t.Fatalf("unexpected public service order: %s, %s", public[0].ServiceCode, public[1].ServiceCode)
	}
}
