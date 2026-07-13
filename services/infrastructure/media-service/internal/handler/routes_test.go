package handler

import (
	"net/http/httptest"
	"testing"
)

func TestValidCallbackToken(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/v1/media/callback/transcode", nil)
	req.Header.Set("X-Media-Callback-Token", "secret")
	if !validCallbackToken(req, "secret") {
		t.Fatal("expected callback token to be accepted")
	}
	if validCallbackToken(req, "other") || validCallbackToken(req, "") {
		t.Fatal("expected callback token to be rejected")
	}
}
