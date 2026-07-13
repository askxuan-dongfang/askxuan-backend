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
