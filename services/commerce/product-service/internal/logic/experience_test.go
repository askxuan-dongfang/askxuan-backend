package logic

import "testing"

func TestExperienceSourceValidation(t *testing.T) {
	for _, tc := range []struct {
		experience bool
		name, url  string
		ok         bool
	}{
		{false, "", "", true}, {true, "案例", "https://example.com/item", true},
		{true, "", "https://example.com/item", false}, {true, "案例", "", false},
		{true, "案例", "javascript:alert(1)", false}, {true, "案例", "https://user:pass@example.com", false},
		{true, "案例", "https://", false}, {false, "", "http://example.com", false},
	} {
		if err := validateExperienceSource(tc.experience, tc.name, tc.url, ""); (err == nil) != tc.ok {
			t.Errorf("%+v got %v", tc, err)
		}
	}
}
