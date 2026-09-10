package agent

import (
	"encoding/json"
	"testing"
	"time"
)

func TestBuildBaziToolArguments(t *testing.T) {
	got, err := BuildToolArguments("bazi", "事业", `{"birthDate":"1990-01-15","birthTime":"09:30","calendarType":"solar","gender":"male","birthplace":"杭州"}`, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]interface{}
	_ = json.Unmarshal([]byte(got), &values)
	if values["birthYear"] != float64(1990) || values["birthHour"] != float64(9) || values["birthPlace"] != "杭州" {
		t.Fatalf("unexpected args: %s", got)
	}
	if redacted := RedactToolArguments(got); redacted == got {
		t.Fatalf("expected sensitive place to be redacted")
	}
}

func TestBuildLiuyaoNumberArguments(t *testing.T) {
	got, err := BuildToolArguments("liuyao", "财运如何", `{"method":"number","numbers":"12, 34, 56","yongShenTarget":"妻财"}`, time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]interface{}
	_ = json.Unmarshal([]byte(got), &values)
	if len(values["numbers"].([]interface{})) != 3 {
		t.Fatalf("unexpected args: %s", got)
	}
}

func TestBuildToolArgumentsWaitsForRequiredProfile(t *testing.T) {
	for _, skill := range []string{"bazi", "ziwei", "qimen"} {
		got, err := BuildToolArguments(skill, "请先帮我看看", `{}`, time.Now())
		if err != nil || got != "" {
			t.Fatalf("%s should wait for structured profile, got %q %v", skill, got, err)
		}
	}
}

func TestDivinationNumbersDoNotSilentlyChangeInput(t *testing.T) {
	for _, value := range []string{"12 34", "0，34、999999", "12, 34, 56"} {
		if _, err := ParseDivinationNumbers(value); err != nil {
			t.Errorf("valid %q: %v", value, err)
		}
	}
	for _, value := range []string{"1", "1 2 3 4", "-1 2", "1.2 3", "1x2", "1000000 2", "999999999999999999 2", "12,34,"} {
		if _, err := ParseDivinationNumbers(value); err == nil {
			t.Errorf("accepted ambiguous %q", value)
		}
	}
}
func TestBirthDatePreservesLunarDayThirty(t *testing.T) {
	for _, code := range []string{"bazi", "ziwei"} {
		got, err := BuildToolArguments(code, "个人方向", `{"birthDate":"1990-02-30","birthTime":"08:30","gender":"male","calendarType":"lunar"}`, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		var values map[string]interface{}
		_ = json.Unmarshal([]byte(got), &values)
		if values["birthDay"] != float64(30) || values["birthMonth"] != float64(2) {
			t.Fatalf("lunar changed: %s", got)
		}
	}
	for _, value := range []string{"1899-01-01", "2101-01-01", "1990-13-01", "1990-02-31"} {
		if _, _, _, err := ParseBirthDate(value, "lunar"); err == nil {
			t.Errorf("accepted %s", value)
		}
	}
	if _, _, _, err := ParseBirthDate("1990-02-30", "solar"); err == nil {
		t.Fatal("accepted invalid Gregorian date")
	}
}
