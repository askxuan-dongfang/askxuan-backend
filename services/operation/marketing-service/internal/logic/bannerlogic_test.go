package logic

import (
	"github.com/askxuan/marketing-service/internal/model"
	"testing"
	"time"
)

func TestMarketingTimeBoundariesAndTimezone(t *testing.T) {
	now := time.Date(2026, 9, 11, 4, 0, 0, 0, time.UTC) // 12:00 Beijing
	for _, tc := range []struct {
		start, end string
		want       bool
	}{
		{"2026-09-11 12:00:00", "2026-09-11 12:00:00", true},
		{"2026-09-11 12:00:01", "", false}, {"", "2026-09-11 11:59:59", false},
		{"2026-09-11", "2026-09-11", true}, {"bad-date", "", false}, {"", "invalid", false}, {"", "", true},
	} {
		if got := inTimeRangeAt(tc.start, tc.end, now); got != tc.want {
			t.Errorf("%+v got %v", tc, got)
		}
	}
}
func TestBannerPublishingValidation(t *testing.T) {
	base := model.Banner{Title: "文化推荐", Placement: "customer_home", ImageUrl: "/assets/banner.jpg", LinkType: "master", LinkValue: "W005", Status: "enabled"}
	for _, tc := range []struct {
		name   string
		change func(*model.Banner)
		valid  bool
	}{
		{"complete", func(*model.Banner) {}, true},
		{"incomplete draft", func(b *model.Banner) { b.Status = "draft"; b.ImageUrl = ""; b.LinkValue = "" }, true},
		{"missing image", func(b *model.Banner) { b.ImageUrl = "" }, false},
		{"insecure image", func(b *model.Banner) { b.ImageUrl = "http://example.com/a.jpg" }, false},
		{"script target", func(b *model.Banner) { b.LinkType = "ad_landing"; b.LinkValue = "javascript:alert(1)" }, false},
		{"unimplemented target", func(b *model.Banner) { b.LinkType = "ad_landing"; b.LinkValue = "/promo/newuser" }, false},
		{"admin target", func(b *model.Banner) { b.LinkType = "ad_landing"; b.LinkValue = "/admin/marketing/banner" }, false},
		{"ai shortcut", func(b *model.Banner) { b.LinkType = "ai"; b.LinkValue = "" }, true},
		{"invalid placement", func(b *model.Banner) { b.Placement = "unknown" }, false},
		{"expired", func(b *model.Banner) { b.EndTime = "2020-01-01" }, false},
		{"inverted", func(b *model.Banner) { b.StartTime = "2099-02-01"; b.EndTime = "2099-01-01" }, false},
		{"empty title", func(b *model.Banner) { b.Title = "  " }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := base
			tc.change(&b)
			if err := validateBanner(b); (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
}
