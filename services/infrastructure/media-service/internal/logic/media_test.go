package logic

import (
	"testing"

	"github.com/askxuan/media-service/internal/model"
	"github.com/askxuan/media-service/internal/types"
)

func TestApplyTranscodeCallbackPreservesOmittedMediaURLs(t *testing.T) {
	media := &model.Media{
		ProviderTaskId: "task-1",
		PlaybackUrl:    "http://media.example/video.mp4",
		CoverUrl:       "http://media.example/cover.jpg",
		Duration:       12,
		ErrorMessage:   "old failure",
	}
	applyTranscodeCallback(media, &types.MediaCallbackReq{Status: "ready"})

	if media.PlaybackUrl == "" || media.CoverUrl == "" || media.Duration != 12 || media.ProviderTaskId != "task-1" {
		t.Fatalf("omitted callback fields must be preserved: %#v", media)
	}
	if media.Status != "ready" || media.ErrorMessage != "" {
		t.Fatalf("callback status was not applied: %#v", media)
	}
}

func TestCanReadMediaEnforcesOwnerAndAuditVisibility(t *testing.T) {
	media := &model.Media{OwnerId: "owner", Status: "ready", AuditStatus: "pending"}
	if !canReadMedia(media, "owner") || canReadMedia(media, "viewer") || canReadMedia(media, "") {
		t.Fatal("pending media visibility is incorrect")
	}
	media.AuditStatus = "approved"
	if !canReadMedia(media, "viewer") || canReadMedia(media, "") {
		t.Fatal("approved media must require an authenticated viewer")
	}
}
