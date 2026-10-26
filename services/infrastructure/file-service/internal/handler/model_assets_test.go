package handler

import (
	"github.com/askxuan/common"
	"net/http/httptest"
	"testing"
)

func TestModelIngressRequiresDiyServiceIdentity(t *testing.T) {
	const secret = "fixture-internal-model-storage-secret"
	request := httptest.NewRequest("PUT", "/internal/diy-models/id/glb", nil)
	request.Header.Set("X-User-Roles", "platform_service")
	if requireDiyService(httptest.NewRecorder(), request, secret) {
		t.Fatal("spoofed internal identity trusted")
	}
	for _, name := range []string{"order-service", "file-service"} {
		token, _ := common.SignServiceToken(secret, name)
		request.Header.Set("Authorization", "Bearer "+token)
		if requireDiyService(httptest.NewRecorder(), request, secret) {
			t.Fatal("other service can upload models")
		}
	}
	admin, _ := common.GenAccessToken(secret, common.TokenInfo{UserId: 1, UserType: "admin", Roles: []string{"platform_super"}}, 3600)
	request.Header.Set("Authorization", "Bearer "+admin)
	if requireDiyService(httptest.NewRecorder(), request, secret) {
		t.Fatal("human token can bypass generation validation")
	}
	token, _ := common.SignServiceToken(secret, "diy-service")
	request.Header.Set("Authorization", "Bearer "+token)
	if !requireDiyService(httptest.NewRecorder(), request, secret) {
		t.Fatal("diy service rejected")
	}
}
