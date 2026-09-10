package logic

import (
	"github.com/askxuan/community-service/internal/types"
	"testing"
)

func TestValidatePostAssetRules(t *testing.T) {
	base := types.PostWriteReq{OwnerId: "1", MasterId: "M1", Title: "标题"}
	base.Type = "video"
	base.Assets = []types.Asset{{MediaId: 1, AssetType: "video"}}
	if validatePost(&base) != nil {
		t.Fatal("valid video rejected")
	}
	base.Assets = append(base.Assets, types.Asset{MediaId: 2, AssetType: "video"})
	if validatePost(&base) == nil {
		t.Fatal("multiple videos accepted")
	}
	base.Type = "article"
	base.Assets = []types.Asset{{MediaId: 1, AssetType: "image"}}
	if validatePost(&base) != nil {
		t.Fatal("valid article rejected")
	}
	base.Assets = append(base.Assets, types.Asset{MediaId: 1, AssetType: "image"})
	if validatePost(&base) == nil {
		t.Fatal("duplicate media accepted")
	}
}

func TestFeedValidation(t *testing.T) {
	for _, tc := range []struct {
		req   types.FeedReq
		valid bool
	}{
		{types.FeedReq{Type: "image"}, true}, {types.FeedReq{Type: "article", Sort: "popular"}, true},
		{types.FeedReq{Type: "wrong"}, false}, {types.FeedReq{Sort: "likes; DROP TABLE post"}, false},
		{types.FeedReq{Following: true}, false}, {types.FeedReq{Following: true, Viewer: "u1"}, true},
	} {
		err := validateFeed(&tc.req)
		if (err == nil) != tc.valid {
			t.Fatalf("%+v: %v", tc.req, err)
		}
		if tc.req.Type == "image" {
			t.Fatal("legacy image alias not normalized")
		}
	}
}
