package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/askxuan/common"
)

func TestRequirePlatformSuperRejectsSpoofedHeader(t *testing.T) {
	const secret = "secure-file-service-signing-key-for-tests"
	request := httptest.NewRequest("GET", "/api/v1/admin/files/backups", nil)
	request.Header.Set("X-User-Roles", "platform_super")
	if requirePlatformSuper(httptest.NewRecorder(), request, secret) {
		t.Fatal("trusted caller-controlled role header")
	}

	serviceToken, err := common.SignServiceToken(secret, "order-service")
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+serviceToken)
	if requirePlatformSuper(httptest.NewRecorder(), request, secret) {
		t.Fatal("service token gained backup administration")
	}

	adminToken, err := common.GenAccessToken(secret, common.TokenInfo{
		UserId: 42, UserType: "admin", Roles: []string{"platform_super"},
	}, 3600)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+adminToken)
	if !requirePlatformSuper(httptest.NewRecorder(), request, secret) {
		t.Fatal("valid platform super token was rejected")
	}
}

func TestUploadRejectsDirectUnauthenticatedAccess(t *testing.T) {
	const secret = "secure-file-service-signing-key-for-tests"
	request := httptest.NewRequest("POST", "/api/v1/files/upload", nil)
	request.Header.Set("X-User-Id", "42")
	request.Header.Set("X-User-Roles", "platform_super")
	if claims := accessClaims(request, secret); claims != nil {
		t.Fatal("trusted spoofed identity headers")
	}
	adminToken, err := common.GenAccessToken(secret, common.TokenInfo{
		UserId: 42, UserType: "admin", Roles: []string{"platform_super"},
	}, 3600)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+adminToken)
	if claims := accessClaims(request, secret); claims == nil || claims.UserId != 42 {
		t.Fatal("rejected valid uploader token")
	}
}
