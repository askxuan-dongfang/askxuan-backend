package logic

import (
	"testing"

	"github.com/askxuan/master-service/internal/model"
)

func TestIsPublicMaster(t *testing.T) {
	visible := &model.Master{AuthStatus: model.MasterAuthStatusVerified, ShelfStatus: model.MasterShelfStatusOnShelf, PlatformStatus: model.MasterPlatformStatusNormal}
	if !isPublicMaster(visible) {
		t.Fatal("verified on-shelf normal master should be public")
	}
	for _, master := range []*model.Master{
		nil,
		{AuthStatus: model.MasterAuthStatusPending, ShelfStatus: model.MasterShelfStatusOnShelf, PlatformStatus: model.MasterPlatformStatusNormal},
		{AuthStatus: model.MasterAuthStatusVerified, ShelfStatus: model.MasterShelfStatusOffShelf, PlatformStatus: model.MasterPlatformStatusNormal},
		{AuthStatus: model.MasterAuthStatusVerified, ShelfStatus: model.MasterShelfStatusOnShelf, PlatformStatus: model.MasterPlatformStatusBanned},
	} {
		if isPublicMaster(master) {
			t.Fatalf("non-public master was exposed: %+v", master)
		}
	}
}
