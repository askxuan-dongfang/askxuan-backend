package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/askxuan/common"
)

func TestResolveUserID(t *testing.T) {
	tests := []struct {
		name      string
		header    string
		requested string
		want      string
		wantErr   error
	}{
		{name: "trusted header", header: "1001", want: "1001"},
		{name: "matching fallback", header: "1001", requested: "1001", want: "1001"},
		{name: "header mismatch", header: "1001", requested: "1002", wantErr: common.ErrForbidden},
		{name: "direct request cannot supply identity", requested: "1001", wantErr: common.ErrForbidden},
		{name: "missing identity", wantErr: common.ErrForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				req.Header.Set("X-User-Id", tt.header)
			}
			got, err := resolveUserID(req, tt.requested)
			if err != tt.wantErr {
				t.Fatalf("resolveUserID() error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("resolveUserID() = %q, want %q", got, tt.want)
			}
		})
	}
}
