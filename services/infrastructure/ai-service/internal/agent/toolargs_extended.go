package agent

import (
	"fmt"
	"math"
	"strconv"
)

func oneOf(s string, values ...string) bool {
	for _, v := range values {
		if s == v {
			return true
		}
	}
	return false
}
func validPalace(s string) bool {
	return oneOf(s, "命宫", "兄弟", "夫妻", "子女", "财帛", "疾厄", "迁移", "仆役", "官禄", "田宅", "福德", "父母")
}
func boundedNumber(in map[string]interface{}, key string, lo, hi float64, integer bool) (float64, error) {
	n, e := strconv.ParseFloat(stringValue(in[key]), 64)
	if e != nil || math.IsNaN(n) || math.IsInf(n, 0) || n < lo || n > hi || (integer && n != math.Trunc(n)) {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return n, nil
}
func extendedToolArguments(code, question string, in map[string]interface{}) (map[string]interface{}, error) {
	a := map[string]interface{}{"detailLevel": "full"}
	switch code {
	case "bazi_pillars_resolve":
		a = map[string]interface{}{}
		for _, k := range []string{"yearPillar", "monthPillar", "dayPillar", "hourPillar"} {
			s := stringValue(in[k])
			if !validPillar(s) {
				return nil, fmt.Errorf("invalid %s", k)
			}
			a[k] = s
		}
	case "astrology":
		y, m, d, e := ParseBirthDate(stringValue(in["astroBirthDate"]), "solar")
		if e != nil {
			return nil, e
		}
		h, min, e := parseClock(stringValue(in["astroBirthTime"]))
		if e != nil {
			return nil, e
		}
		lat, e := boundedNumber(in, "latitude", -90, 90, false)
		if e != nil {
			return nil, e
		}
		lng, e := boundedNumber(in, "longitude", -180, 180, false)
		if e != nil {
			return nil, e
		}
		transit, e := parseLocalDateTime(stringValue(in["transitDateTime"]))
		if e != nil {
			return nil, e
		}
		a["birthYear"], a["birthMonth"], a["birthDay"], a["birthHour"], a["birthMinute"] = y, m, d, h, min
		a["latitude"], a["longitude"], a["transitDateTime"], a["houseSystem"] = lat, lng, transit.UTC().Format("2006-01-02T15:04:05Z"), "placidus"
	case "almanac":
		date := stringValue(in["targetDate"])
		if _, _, _, e := ParseBirthDate(date, "solar"); e != nil {
			return nil, e
		}
		a = map[string]interface{}{"date": date}
		if master := stringValue(in["dayMaster"]); master != "" {
			if !oneOf(master, "甲", "乙", "丙", "丁", "戊", "己", "庚", "辛", "壬", "癸") {
				return nil, fmt.Errorf("invalid day master")
			}
			a["dayMaster"] = master
		}
	case "xiaoliuren":
		for _, v := range []struct {
			k, dest string
			lo, hi  float64
		}{{"lunarMonth", "lunarMonth", 1, 12}, {"lunarDay", "lunarDay", 1, 30}, {"hourIndex", "hour", 1, 12}} {
			n, e := boundedNumber(in, v.k, v.lo, v.hi, true)
			if e != nil {
				return nil, e
			}
			a[v.dest] = n
		}
		a["question"] = question
	case "taiyi", "daliuren", "meihua":
		t, e := parseLocalDateTime(stringValue(in["eventTime"]))
		if e != nil {
			return nil, e
		}
		a["question"] = question
		if code != "meihua" {
			a["date"], a["hour"], a["minute"], a["timezone"] = t.Format("2006-01-02"), t.Hour(), t.Minute(), "Asia/Shanghai"
		}
		if code == "taiyi" {
			mode := stringValue(in["taiyiMode"])
			if !oneOf(mode, "year", "month", "day", "hour", "minute") {
				return nil, fmt.Errorf("invalid taiyi mode")
			}
			a["mode"] = mode
		}
		if code == "meihua" {
			method := stringValue(in["meihuaMethod"])
			a["date"], a["method"] = t.Format("2006-01-02T15:04:05"), method
			switch method {
			case "time":
			case "number_pair", "number_triplet":
				key, count := "pairNumbers", 2
				if method == "number_triplet" {
					key, count = "tripleNumbers", 3
				}
				ns, e := ParseDivinationNumbers(stringValue(in[key]))
				if e != nil || len(ns) != count {
					return nil, fmt.Errorf("invalid meihua numbers")
				}
				a["numbers"] = ns
			case "text_split":
				text := stringValue(in["divinationText"])
				if text == "" {
					return nil, fmt.Errorf("text required")
				}
				a["text"], a["textSplitMode"] = text, "auto"
			case "count_with_time":
				n, e := boundedNumber(in, "count", 1, 999999, true)
				if e != nil {
					return nil, e
				}
				kind := stringValue(in["countCategory"])
				if !oneOf(kind, "item", "sound") {
					return nil, fmt.Errorf("count category required")
				}
				a["count"], a["countCategory"] = n, kind
			case "measure":
				kind := stringValue(in["measureKind"])
				if !oneOf(kind, "丈尺", "尺寸") {
					return nil, fmt.Errorf("invalid measure kind")
				}
				a["measureKind"] = kind
				for _, k := range []string{"majorValue", "minorValue"} {
					n, e := boundedNumber(in, k, 0, 999999, true)
					if e != nil {
						return nil, e
					}
					a[k] = n
				}
			case "classifier_pair":
				for _, k := range []string{"upperCue", "lowerCue"} {
					v := stringValue(in[k])
					if v == "" {
						return nil, fmt.Errorf("%s required", k)
					}
					a[k] = v
				}
			case "select":
				name := stringValue(in["hexagramName"])
				if name == "" {
					return nil, fmt.Errorf("hexagram required")
				}
				n, e := boundedNumber(in, "movingLine", 1, 6, true)
				if e != nil {
					return nil, e
				}
				a["hexagramName"], a["movingLine"] = name, n
			default:
				return nil, fmt.Errorf("invalid meihua method")
			}
		}
	default:
		return nil, nil
	}
	return a, nil
}
