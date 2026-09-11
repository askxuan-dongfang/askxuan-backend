package types

import (
	"github.com/zeromicro/go-zero/rest/httpx"
	"github.com/zeromicro/go-zero/rest/pathvar"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBannerHTTPPartialUpdate(t *testing.T) {
	for _, body := range []string{`{"status":"disabled"}`, `{"sort":0,"startTime":"","endTime":""}`} {
		r := httptest.NewRequest("PUT", "/api/v1/admin/marketing/banners/9", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r = pathvar.WithVars(r, map[string]string{"id": "9"})
		var req BannerUpdateReq
		if err := httpx.Parse(r, &req); err != nil {
			t.Fatal(err)
		}
		if req.Id != 9 {
			t.Fatal("missing id")
		}
		if req.Status != nil {
			if req.Sort != nil || req.StartTime != nil || req.EndTime != nil || req.LinkValue != nil {
				t.Fatal("omitted values were populated")
			}
		} else if req.Sort == nil || *req.Sort != 0 || req.StartTime == nil || *req.StartTime != "" || req.EndTime == nil || *req.EndTime != "" {
			t.Fatal("explicit zero/empty missing")
		}
	}
}
