package agent

import (
	"encoding/json"
	"testing"
	"time"
)

func TestAstrologyArgumentsPreserveBeijingClockWithExplicitOffset(t *testing.T) {
	for _, coordinates := range [][2]string{{"43.799783", "87.595204"}, {"30.259287", "120.125628"}, {"40.7128", "-74.006"}} {
		input, _ := json.Marshal(map[string]any{"astroBirthDate": "1992-05-18", "astroBirthTime": "00:30", "latitude": coordinates[0], "longitude": coordinates[1], "transitDateTime": "2026-11-01T14:30"})
		raw, err := BuildToolArguments("astrology", "合成时区验证", string(input), time.Time{})
		if err != nil {
			t.Fatal(err)
		}
		var args map[string]any
		if err := json.Unmarshal([]byte(raw), &args); err != nil {
			t.Fatal(err)
		}
		if args["birthYear"] != float64(1992) || args["birthMonth"] != float64(5) || args["birthDay"] != float64(18) || args["birthHour"] != float64(0) || args["birthMinute"] != float64(30) || args["birthUtcOffsetMinutes"] != float64(480) {
			t.Fatalf("birth clock or source UTC offset lost: %s", raw)
		}
		if args["transitDateTime"] != "2026-11-01T06:30:00Z" || args["houseSystem"] != "placidus" {
			t.Fatalf("transit instant lost before a New York fold: %s", raw)
		}
		if _, ok := args["timeBasis"]; ok {
			t.Fatal("astrology incorrectly joined the true-solar-time tools")
		}
	}
}
