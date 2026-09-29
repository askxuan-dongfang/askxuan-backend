package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func invokeTopic(t *testing.T, c *MCPClient, code, inputs string) TopicEvidence {
	t.Helper()
	raw, e := c.Call(context.Background(), `{"enabled":true,"server":"builtin","tool":"`+code+`"}`, inputs)
	if e != nil {
		t.Fatal(e)
	}
	var out TopicEvidence
	if e = json.Unmarshal([]byte(raw), &out); e != nil {
		t.Fatal(e)
	}
	return out
}
func TestNamingUsesDictionaryAndFlagsActualInputs(t *testing.T) {
	out := invokeTopic(t, nil, "naming", `{"mode":"review","surname":"单","name":"单乐乐，单宁","avoidChars":"乐"}`)
	if len(out.Rows) != 2 || out.Rows[0][1] != "3" || !strings.Contains(out.Rows[0][2], "lè") || !strings.Contains(out.Rows[0][3], "多音字") || !strings.Contains(out.Rows[0][3], "重复字") || !strings.Contains(out.Rows[0][3], "避用字") {
		t.Fatalf("dictionary/constraint data missing: %+v", out)
	}
	for _, raw := range []string{`{"mode":"review","surname":"李"}`, `{"mode":"review","surname":"李","name":"1,2,3,4,5,6,7,8,9"}`, `{"mode":"review","surname":"李","name":"李明","url":"http://attacker"}`} {
		if _, e := (*MCPClient)(nil).Call(context.Background(), `{"enabled":true,"server":"builtin","tool":"naming"}`, raw); e == nil {
			t.Fatal("invalid/unbound input accepted")
		}
	}
}
func TestReflectionPreservesUnknownAndDoesNotInventObservation(t *testing.T) {
	s := invokeTopic(t, nil, "fengshui", `{"scene":"home","daylight":"dim"}`)
	if s.Rows[0][2] != "感觉偏暗" || s.Rows[1][2] != "未确认" {
		t.Fatal(s)
	}
	d := invokeTopic(t, nil, "dream", `{"dream":"我梦见红色的门。","feeling":"好奇"}`)
	if d.Rows[0][2] != "我梦见红色的门。" || d.Rows[2][2] != "未填写" || !strings.Contains(d.Note, "不是诊断") {
		t.Fatal(d)
	}
}
func TestDateRangeBoundsDedupAndWeekdays(t *testing.T) {
	for _, in := range []map[string]any{{"targetDate": "2026-09-29", "dateMode": "range", "endDate": "2026-10-13"}, {"targetDate": "2026-09-29", "dateMode": "range", "endDate": "2026-09-28"}, {"targetDate": "2026-02-30"}, {"targetDate": "2026-10-03", "weekdayOnly": "weekdays"}} {
		if _, e := comparisonDates(in); e == nil {
			t.Fatalf("invalid dates accepted: %+v", in)
		}
	}
	ds, e := comparisonDates(map[string]any{"targetDate": "2026-09-29", "secondDate": "2026-09-29", "thirdDate": "2026-09-30"})
	if e != nil || len(ds) != 2 {
		t.Fatalf("%v %v", ds, e)
	}
	ds, e = comparisonDates(map[string]any{"targetDate": "2026-09-29", "dateMode": "range", "endDate": "2026-10-12"})
	if e != nil || len(ds) != 14 {
		t.Fatal(ds, e)
	}
}
func TestDateComparisonUsesActualAlmanacAndFailsClosed(t *testing.T) {
	calls := 0
	fail := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if fail {
			http.Error(w, "down", 503)
			return
		}
		var req struct {
			Params struct {
				Name      string
				Arguments map[string]any
			}
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Params.Name != "almanac" {
			t.Error("wrong dependency")
		}
		date := req.Params.Arguments["date"]
		terms := map[string]any{"宜": []string{"嫁娶"}, "忌": []string{}}
		if date == "2026-09-30" {
			terms["忌"] = []string{"嫁娶"}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": map[string]any{"structuredContent": map[string]any{"择日宜忌": terms, "基础与个性化坐标": map[string]any{"日期": date}}}})
	}))
	defer srv.Close()
	c := NewMCPClient(true, srv.URL, 1)
	in := `{"event":"婚礼","targetDate":"2026-09-29","dateMode":"range","endDate":"2026-09-30"}`
	out := invokeTopic(t, c, "date_select", in)
	if calls != 2 || len(out.Calculations) != 2 || out.Rows[0][2] != "宜：嫁娶" || out.Rows[1][2] != "宜忌同时列出，需核对" {
		t.Fatal(out, calls)
	}
	fail = true
	if raw, e := c.Call(context.Background(), `{"enabled":true,"server":"builtin","tool":"date_select"}`, in); e == nil || raw != "" {
		t.Fatal("partial or fabricated success")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	before := calls
	if _, e := c.Call(ctx, `{"enabled":true,"server":"builtin","tool":"date_select"}`, in); e == nil || calls != before {
		t.Fatal("cancel did not stop remote calls")
	}
}
func TestPersonalFortuneBindsComputedDayMaster(t *testing.T) {
	calls := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Name      string
				Arguments map[string]any
			}
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		calls = append(calls, req.Params.Name)
		if req.Params.Name == "bazi" {
			fmt.Fprint(w, `{"result":{"structuredContent":{"基本信息":{"日主":"丁"}}}}`)
			return
		}
		if req.Params.Arguments["dayMaster"] != "丁" {
			t.Error("day master was guessed or discarded")
		}
		fmt.Fprint(w, `{"result":{"structuredContent":{"基础与个性化坐标":{"日期":"2026-09-29","日干支":"丙午","流日十神":"劫财"}}}}`)
	}))
	defer srv.Close()
	c := NewMCPClient(true, srv.URL, 1)
	out := invokeTopic(t, c, "fortune", `{"targetDate":"2026-09-29","fortuneMode":"personal","calendarType":"solar","birthDate":"1990-01-02","birthTime":"08:30","gender":"male"}`)
	if strings.Join(calls, ",") != "bazi,almanac" || out.Rows[0][1] != "丁" || len(out.Calculations) != 2 {
		t.Fatal(out, calls)
	}
	before := len(calls)
	if _, e := c.Call(context.Background(), `{"enabled":true,"server":"builtin","tool":"fortune"}`, `{"targetDate":"2026-09-29","fortuneMode":"personal"}`); e == nil || len(calls) != before {
		t.Fatal("missing profile did not stop computation")
	}
}
func TestBuiltinReadinessDoesNotPretendRemoteIsAvailable(t *testing.T) {
	c := NewMCPClient(false, "", 1)
	if !c.Configured(ToolConfig{Enabled: true, Server: "builtin", Tool: "naming"}) || c.Configured(ToolConfig{Enabled: true, Server: "builtin", Tool: "date_select"}) || c.Configured(ToolConfig{Enabled: true, Server: "builtin", Tool: "shell"}) {
		t.Fatal("invalid readiness")
	}
}

func TestDateComparisonRejectsMismatchedAndMalformedEvidence(t *testing.T) {
	for _, result := range []string{
		`{"基础与个性化坐标":{"日期":"2026-09-30"},"择日宜忌":{"宜":["嫁娶"],"忌":[]}}`,
		`{"基础与个性化坐标":{"日期":"2026-09-29"},"择日宜忌":{"宜":"嫁娶","忌":[]}}`,
		`{"基础与个性化坐标":{"日期":"2026-09-29"},"择日宜忌":{"宜":[123],"忌":[]}}`,
	} {
		t.Run(result, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"result":{"structuredContent":%s}}`, result)
			}))
			defer srv.Close()
			c := NewMCPClient(true, srv.URL, 1)
			raw, err := c.Call(context.Background(), `{"enabled":true,"server":"builtin","tool":"date_select"}`, `{"event":"嫁娶","targetDate":"2026-09-29"}`)
			if err == nil || raw != "" {
				t.Fatal("invalid provenance published as a comparison", raw, err)
			}
		})
	}
}
