package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/askxuan/ai-service/internal/types"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func TestSessionCreateRequestParsesOptionalImageDimensions(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ai/sessions", strings.NewReader(`{
		"skillCode":"general",
		"question":"请说明图片内容",
		"attachments":[{
			"mediaId":9,
			"url":"https://eliaukcloud.cn/askxuan-media/ai/test.png",
			"contentType":"image/png"
		}]
	}`))
	req.Header.Set("Content-Type", "application/json")

	var parsed types.SessionCreateReq
	if err := httpx.Parse(req, &parsed); err != nil {
		t.Fatalf("parse session create request: %v", err)
	}
	if len(parsed.Attachments) != 1 || parsed.Attachments[0].MediaId != 9 {
		t.Fatalf("unexpected attachments: %#v", parsed.Attachments)
	}
}

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
