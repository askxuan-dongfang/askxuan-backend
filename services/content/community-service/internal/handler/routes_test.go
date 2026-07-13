package handler

import (
	"net/http/httptest"
	"testing"
)

func TestRequirePlatformRole(t *testing.T) {
	request := httptest.NewRequest("GET", "/api/v1/admin/platform/community/posts", nil)
	request.Header.Set("X-User-Id", "temple-admin")
	request.Header.Set("X-User-Roles", "temple_admin")
	if requirePlatformRole(request) == nil {
		t.Fatal("temple admin must not review community content")
	}
	request.Header.Set("X-User-Roles", "platform_service")
	if err := requirePlatformRole(request); err != nil {
		t.Fatalf("platform service role rejected: %v", err)
	}
}
