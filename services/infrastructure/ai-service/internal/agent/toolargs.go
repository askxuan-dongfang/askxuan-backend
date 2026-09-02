package agent

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var numberPattern = regexp.MustCompile(`\d+`)

func BuildToolArguments(skillCode, question, inputJSON string, now time.Time) (string, error) {
	inputs := map[string]interface{}{}
	if strings.TrimSpace(inputJSON) != "" {
		if err := json.Unmarshal([]byte(inputJSON), &inputs); err != nil {
			return "", fmt.Errorf("decode tool inputs: %w", err)
		}
	}
	var args map[string]interface{}
	switch skillCode {
	case "bazi", "ziwei":
		if stringValue(inputs["birthDate"]) == "" || stringValue(inputs["birthTime"]) == "" || stringValue(inputs["gender"]) == "" {
			return "", nil
		}
		date, err := time.Parse("2006-01-02", stringValue(inputs["birthDate"]))
		if err != nil {
			return "", fmt.Errorf("birthDate must be YYYY-MM-DD")
		}
		hour, minute, err := parseClock(stringValue(inputs["birthTime"]))
		if err != nil {
			return "", err
		}
		args = map[string]interface{}{
			"gender": stringValue(inputs["gender"]), "birthYear": date.Year(), "birthMonth": int(date.Month()), "birthDay": date.Day(),
			"birthHour": hour, "birthMinute": minute, "calendarType": defaultString(stringValue(inputs["calendarType"]), "solar"), "detailLevel": "default",
		}
		if skillCode == "bazi" && stringValue(inputs["birthplace"]) != "" {
			args["birthPlace"] = stringValue(inputs["birthplace"])
		}
	case "qimen":
		if stringValue(inputs["eventTime"]) == "" {
			return "", nil
		}
		eventTime, err := parseLocalDateTime(stringValue(inputs["eventTime"]))
		if err != nil {
			return "", err
		}
		args = map[string]interface{}{
			"year": eventTime.Year(), "month": int(eventTime.Month()), "day": eventTime.Day(), "hour": eventTime.Hour(), "minute": eventTime.Minute(),
			"timezone": "Asia/Shanghai", "question": question, "panType": "zhuan", "juMethod": "chaibu", "zhiFuJiGong": "ji_liuyi", "detailLevel": "default",
		}
	case "tarot":
		spread := map[string]string{"single": "single", "three": "three-card", "love": "love", "decision": "decision"}[stringValue(inputs["spread"])]
		if spread == "" {
			spread = "single"
		}
		args = map[string]interface{}{"spreadType": spread, "question": question, "allowReversed": true, "detailLevel": "default"}
	case "liuyao":
		method := defaultString(stringValue(inputs["method"]), "auto")
		args = map[string]interface{}{
			"question": question, "method": method, "yongShenTargets": []string{defaultString(stringValue(inputs["yongShenTarget"]), inferYongShen(question))},
			"date": defaultString(stringValue(inputs["eventTime"]), now.In(time.FixedZone("CST", 8*3600)).Format("2006-01-02T15:04:05")), "detailLevel": "default",
		}
		if method == "number" {
			raw := numberPattern.FindAllString(stringValue(inputs["numbers"]), 3)
			if len(raw) < 2 {
				return "", fmt.Errorf("number method requires 2-3 numbers")
			}
			numbers := make([]int, 0, len(raw))
			for _, value := range raw {
				parsed, _ := strconv.Atoi(value)
				numbers = append(numbers, parsed)
			}
			args["numbers"] = numbers
		}
	default:
		return "", nil
	}
	encoded, err := json.Marshal(args)
	return string(encoded), err
}

func RedactToolArguments(argumentsJSON string) string {
	values := map[string]interface{}{}
	if json.Unmarshal([]byte(argumentsJSON), &values) != nil {
		return `{}`
	}
	for _, key := range []string{"birthPlace", "location", "question"} {
		if _, ok := values[key]; ok {
			values[key] = "[已提供]"
		}
	}
	encoded, _ := json.Marshal(values)
	return string(encoded)
}

func parseClock(value string) (int, int, error) {
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return 0, 0, fmt.Errorf("birthTime must be HH:mm")
	}
	return parsed.Hour(), parsed.Minute(), nil
}

func parseLocalDateTime(value string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02T15:04", "2006-01-02T15:04:05", "2006-01-02 15:04", "2006-01-02 15:04:05"} {
		if parsed, err := time.ParseInLocation(layout, value, time.FixedZone("CST", 8*3600)); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("eventTime must be a local date-time")
}

func inferYongShen(question string) string {
	for keyword, target := range map[string]string{"事业": "官鬼", "工作": "官鬼", "财": "妻财", "投资": "妻财", "健康": "子孙", "考试": "父母", "合同": "父母", "合作": "兄弟", "竞争": "兄弟"} {
		if strings.Contains(question, keyword) {
			return target
		}
	}
	return "官鬼"
}

func stringValue(value interface{}) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
