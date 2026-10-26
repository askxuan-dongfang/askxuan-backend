package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSolarArgumentsReachEveryTimeTool(t *testing.T) {
	inputs := map[string]any{"birthDate": "1992-05-18", "birthTime": "00:30", "gender": "male", "calendarType": "solar", "birthplace": "乌鲁木齐市", "location": "乌鲁木齐市", "longitude": "87.6168", "timeBasis": "true_solar", "targetDate": "2026-10-27", "eventTime": "2026-10-26T00:30", "method": "time", "yongShenTarget": "官鬼", "queryType": "selfMutaged", "palace": "命宫", "meihuaMethod": "time", "taiyiMode": "hour"}
	for _, code := range []string{"bazi", "bazi_dayun", "ziwei", "ziwei_horoscope", "ziwei_flying_star", "qimen", "liuyao", "meihua", "daliuren", "taiyi"} {
		raw, _ := json.Marshal(inputs)
		args, err := BuildToolArguments(code, "核对跨日排盘", string(raw), time.Now())
		if err != nil {
			t.Fatalf("%s: %v", code, err)
		}
		var got map[string]any
		_ = json.Unmarshal([]byte(args), &got)
		if got["longitude"] != 87.6168 || got["birthPlace"] != "乌鲁木齐市" || got["timeBasis"] != "true_solar" {
			t.Fatalf("%s lost location: %s", code, args)
		}
		if got["birthHour"] != nil && got["birthHour"] != float64(0) {
			t.Fatalf("%s corrected time twice", code)
		}
	}
}

func TestSolarInputValidationAndClockCompatibility(t *testing.T) {
	base := `"birthDate":"1992-05-18","birthTime":"00:30","gender":"male"`
	for _, extra := range []string{`,"timeBasis":"true_solar"`, `,"timeBasis":"true_solar","longitude":"NaN"`, `,"longitude":"181"`, `,"timeBasis":"invented"`} {
		if _, err := BuildToolArguments("bazi", "排盘", "{"+base+extra+"}", time.Now()); err == nil {
			t.Fatalf("accepted %s", extra)
		}
	}
	args, err := BuildToolArguments("bazi", "排盘", "{"+base+`,"timeBasis":"beijing","birthplace":"杭州市","longitude":"120.15"}`, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	var clock map[string]any
	_ = json.Unmarshal([]byte(args), &clock)
	if clock["birthPlace"] != nil || clock["longitude"] != nil || clock["timeBasis"] != "beijing" {
		t.Fatal(args)
	}
	for _, code := range []string{"bazi", "bazi_dayun", "ziwei", "qimen", "marriage", "fortune"} {
		var schema InputSchema
		_ = json.Unmarshal([]byte(GuidedInputSchema(code, `{"fields":[]}`)), &schema)
		found := map[string]Field{}
		for _, f := range schema.Fields {
			found[f.Key] = f
		}
		if found["timeBasis"].DefaultValue != "true_solar" || found["longitude"].Key != "" || found["partnerLongitude"].Key != "" {
			t.Fatalf("%s missing time guidance", code)
		}
		if code == "marriage" && found["partnerBirthplace"].Key == "" {
			t.Fatal("partner place missing")
		}
		if code == "fortune" && found["timeBasis"].VisibleWhen == nil {
			t.Fatal("general fortune must not ask birth location")
		}
	}
}

func TestPersonalFortuneKeepsBirthLocation(t *testing.T) {
	seen := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		data := map[string]any{"基础与个性化坐标": map[string]any{"日期": "2026-10-27", "日干支": "甲戌", "流日十神": "比肩"}}
		if req.Params.Name == "bazi" {
			seen = true
			if req.Params.Arguments["longitude"] != 87.6168 || req.Params.Arguments["timeBasis"] != "true_solar" {
				t.Error("personal fortune lost solar basis")
			}
			data = map[string]any{"基本信息": map[string]any{"日主": "甲"}}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"structuredContent": data}})
	}))
	defer srv.Close()
	c := NewMCPClient(true, srv.URL, 2)
	_, err := c.fortuneEvidence(context.Background(), map[string]any{"fortuneMode": "personal", "targetDate": "2026-10-27", "birthDate": "1992-05-18", "birthTime": "00:30", "gender": "male", "longitude": "87.6168", "timeBasis": "true_solar"})
	if err != nil || !seen {
		t.Fatalf("birth calculation: %v", err)
	}
}
