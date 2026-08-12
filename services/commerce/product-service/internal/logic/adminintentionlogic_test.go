package logic

import "testing"

func TestValidLandingType(t *testing.T) {
	for _, value := range []string{"", "aggregate", "service", "diy"} {
		if !validLandingType(value) {
			t.Fatalf("expected %q to be valid", value)
		}
	}
	for _, value := range []string{"product", "url", "SERVICE"} {
		if validLandingType(value) {
			t.Fatalf("expected %q to be invalid", value)
		}
	}
}

func TestRequestToIntentTagDefaults(t *testing.T) {
	tag := requestToIntentTag("health", "求健康", "", "", "", "", "", 10)
	if tag.Icon != "sparkles" || tag.LandingType != "aggregate" || tag.Status != "enabled" {
		t.Fatalf("unexpected defaults: %+v", tag)
	}
}
