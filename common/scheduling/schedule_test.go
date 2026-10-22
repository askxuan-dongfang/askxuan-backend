package scheduling

import "testing"

func TestScheduleBoundaries(t *testing.T) {
	for _, tc := range []struct {
		status string
		slots  []string
		time   string
		want   bool
	}{
		{"off", nil, "09:00-10:00", false}, {"available", []string{"09:00-12:00"}, "09:00-12:00", true}, {"available", []string{"09:00-12:00"}, "08:59-10:00", false}, {"available", []string{"09:00-12:00", "13:00-17:00"}, "11:00-14:00", false}, {"booked", []string{"09:00-12:00"}, "09:00-10:00", false}} {
		if Allows(tc.status, tc.slots, tc.time) != tc.want {
			t.Errorf("%+v", tc)
		}
	}
	for _, slots := range [][]string{{"24:00-25:00"}, {"12:00-09:00"}, {"09:00-12:00", "11:00-13:00"}, {}} {
		if Validate("2026-10-22", "available", slots) == nil {
			t.Fatalf("accepted invalid %v", slots)
		}
	}
	if Validate("2026-02-30", "off", nil) == nil {
		t.Fatal("invalid date")
	}
	if Validate("2026-10-22", "off", nil) != nil {
		t.Fatal("rest day")
	}
}
