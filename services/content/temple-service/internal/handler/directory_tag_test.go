package handler

import (
	"github.com/askxuan/temple-service/internal/model"
	"github.com/zeromicro/go-zero/rest/httpx"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDirectoryTagBodyParsing(t *testing.T) {
	var req struct {
		model.DirectoryTag
		PathCode string `path:"code,optional"`
	}
	r := httptest.NewRequest("POST", "/api/v1/admin/platform/beliefs/directory-tags", strings.NewReader(`{"code":"mazu","group":"tradition","name":"妈祖","value":"mazu","sort":10,"status":"enabled"}`))
	r.Header.Set("Content-Type", "application/json")
	if err := httpx.Parse(r, &req); err != nil {
		t.Fatal(err)
	}
	if req.Code != "mazu" || req.Group != "tradition" || req.Status != "enabled" {
		t.Fatalf("body was not parsed: %+v", req)
	}
}
