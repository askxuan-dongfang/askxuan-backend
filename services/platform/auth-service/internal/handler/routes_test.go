package handler

import (
	"github.com/askxuan/common"
	"net/http/httptest"
	"testing"
)

func TestResolveOpenIMUserID(t *testing.T) {
	tests := []struct {
		name     string
		userID   string
		masterID string
		want     string
	}{
		{name: "customer", userID: "1", want: "u_1"},
		{name: "master identity takes precedence", userID: "3", masterID: "1", want: "m_1"},
		{name: "missing identity", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveOpenIMUserID(tt.userID, tt.masterID); got != tt.want {
				t.Fatalf("resolveOpenIMUserID(%q, %q) = %q, want %q", tt.userID, tt.masterID, got, tt.want)
			}
		})
	}
}

func TestIMIdentityRejectsForgedHeaders(t *testing.T) {
	secret := "isolated-test-secret"
	r := httptest.NewRequest("POST", "/api/v1/auth/im-token", nil)
	r.Header.Set("X-User-Id", "88")
	r.Header.Set("X-Master-Id", "7")
	if _, err := authenticatedIMIdentity(r, secret); err == nil {
		t.Fatal("identity headers bypassed authentication")
	}
	token, err := common.GenAccessToken(secret, common.TokenInfo{UserId: 42, UserType: "user"}, 60)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Authorization", "Bearer "+token)
	id, err := authenticatedIMIdentity(r, secret)
	if err != nil || id != "u_42" {
		t.Fatalf("forged master header changed identity: %s %v", id, err)
	}
	token, _ = common.GenRefreshToken(secret, 42, 60)
	r.Header.Set("Authorization", "Bearer "+token)
	if _, err = authenticatedIMIdentity(r, secret); err == nil {
		t.Fatal("refresh token issued an IM credential")
	}
}
