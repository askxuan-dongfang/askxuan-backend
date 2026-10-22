package handler

import (
	"github.com/askxuan/temple-service/internal/model"
	"github.com/zeromicro/go-zero/rest/httpx"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTempleStatePatchJSON(t *testing.T) {
	var req struct {
		Id string `path:"id"`
		model.TempleStatePatch
	}
	r := httptest.NewRequest("PUT", "/api/v1/admin/platform/temples/T001/state", strings.NewReader(`{"version":42,"platformStatus":"banned","shelfStatus":"off_shelf","recommended":false}`))
	r.Header.Set("Content-Type", "application/json")
	if err := httpx.ParseJsonBody(r, &req); err != nil {
		t.Fatal(err)
	}
	if req.Version != 42 || req.PlatformStatus == nil || *req.PlatformStatus != "banned" || req.Recommended == nil || *req.Recommended {
		t.Fatalf("lost patch fields: %+v", req)
	}
}
