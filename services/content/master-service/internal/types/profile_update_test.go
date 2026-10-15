package types

import (
	"github.com/zeromicro/go-zero/rest/httpx"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProfileOptionalClear(t *testing.T) {
	for _, body := range []string{`{}`, `{"avatar":"","bio":"","pricing":"","specialties":[]}`} {
		request := httptest.NewRequest("PUT", "/admin/masters/profile", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		var got MasterProfileUpdateReq
		if err := httpx.Parse(request, &got); err != nil {
			t.Fatal(err)
		}
		if body == `{}` {
			if got.Avatar != nil || got.Bio != nil || got.Pricing != nil || got.Specialties != nil {
				t.Fatal("omitted fields must be nil")
			}
		} else {
			if got.Avatar == nil || *got.Avatar != "" || got.Bio == nil || *got.Bio != "" || got.Pricing == nil || *got.Pricing != "" || got.Specialties == nil {
				t.Fatal("explicit empty must clear")
			}
		}
	}
}
