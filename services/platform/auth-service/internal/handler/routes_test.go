package handler

import "testing"

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
