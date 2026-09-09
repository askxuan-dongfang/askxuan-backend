package types

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zeromicro/go-zero/rest/httpx"
	"github.com/zeromicro/go-zero/rest/pathvar"
)

func TestCouponHTTPPartialUpdate(t *testing.T) {
	for _, body := range []string{`{"status":"disabled"}`, `{"value":0,"minAmount":0,"totalCount":0,"categoryId":""}`} {
		t.Run(body, func(t *testing.T) {
			r := httptest.NewRequest("PUT", "/api/v1/admin/marketing/coupons/9", strings.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			r = pathvar.WithVars(r, map[string]string{"id": "9"})
			var req CouponUpdateReq
			if err := httpx.Parse(r, &req); err != nil {
				t.Fatal(err)
			}
			if req.Id != 9 {
				t.Fatal("missing coupon id")
			}
			if req.Status == "disabled" {
				if req.Value != nil || req.MinAmount != nil || req.TotalCount != nil || req.CategoryId != nil {
					t.Fatal("omitted fields must remain absent")
				}
			} else if req.Value == nil || *req.Value != 0 || req.MinAmount == nil || *req.MinAmount != 0 || req.TotalCount == nil || *req.TotalCount != 0 || req.CategoryId == nil || *req.CategoryId != "" {
				t.Fatal("explicit zero or empty values must remain present")
			}
		})
	}
}
