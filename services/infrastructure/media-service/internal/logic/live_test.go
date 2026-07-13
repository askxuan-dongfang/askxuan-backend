package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/askxuan/media-service/internal/model"
	"github.com/askxuan/media-service/internal/provider"
	"github.com/askxuan/media-service/internal/svc"
	"github.com/askxuan/media-service/internal/types"
)

func TestLiveCapabilitiesDisabledWithoutCloudProvider(t *testing.T) {
	ctx := &svc.ServiceContext{LiveEnabled: false, LiveProvider: provider.DisabledLiveProvider{ProviderName: "disabled"}}
	capabilities := LiveCapabilities(ctx)
	if capabilities.Enabled || capabilities.Configured || capabilities.CanStart {
		t.Fatalf("unexpected live capabilities: %#v", capabilities)
	}
}

func TestStartLiveRoomRejectsDisabledProviderBeforeDatabaseAccess(t *testing.T) {
	ctx := &svc.ServiceContext{LiveEnabled: true, LiveProvider: provider.DisabledLiveProvider{ProviderName: "local_dev"}}
	_, err := StartLiveRoom(context.Background(), ctx, &types.LiveRoomActionReq{Id: 1, OwnerId: "1"})
	if err == nil {
		t.Fatal("expected disabled live provider error")
	}
}

func TestDisabledLiveProviderNeverReturnsFakeSession(t *testing.T) {
	_, err := (provider.DisabledLiveProvider{}).Start(context.Background(), "LIVE001")
	if !errors.Is(err, provider.ErrLiveUnavailable) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCanViewLiveRoomProtectsDraftAndEndedRooms(t *testing.T) {
	room := &model.LiveRoom{OwnerId: "owner", Status: "created"}
	if !canViewLiveRoom(room, "owner") || canViewLiveRoom(room, "viewer") || canViewLiveRoom(room, "") {
		t.Fatal("draft room visibility is incorrect")
	}
	room.Status = "live"
	if !canViewLiveRoom(room, "viewer") || canViewLiveRoom(room, "") {
		t.Fatal("live room must require an authenticated viewer")
	}
	room.Status = "ended"
	if canViewLiveRoom(room, "viewer") {
		t.Fatal("ended room must not be visible to other users")
	}
}
