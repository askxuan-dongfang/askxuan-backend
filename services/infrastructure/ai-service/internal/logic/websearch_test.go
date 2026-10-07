package logic

import (
	"github.com/askxuan/ai-service/internal/config"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/ai-service/internal/websearch"
	"strings"
	"testing"
)

func TestWebSearchPermissionAndRetryIsolation(t *testing.T) {
	s := &svc.ServiceContext{AIConfig: config.AIConf{HarnessEnabled: true, WebSearch: websearch.Config{Provider: "tavily", APIKey: "test"}}, WebSearchEnabled: true}
	raw, e := withWebSearch(s, `{"birthDate":"1990-01-01"}`, true)
	if e != nil {
		t.Fatal(e)
	}
	messages := []*model.AIMessage{{Id: 1, Role: "user", Status: "completed", Content: "找原文", InputJSON: raw}, {Id: 2, Role: "assistant", Status: "pending"}, {Id: 3, Role: "user", Status: "completed", Content: "不用联网", InputJSON: `{}`}}
	in, e := liveContext(messages, 2, 10, nil)
	if e != nil || !in.WebSearchRequested || in.Facts["_webSearch"] != nil {
		t.Fatal("retry lost choice or leaked metadata")
	}
	in, e = liveContext(messages, 4, 10, nil)
	if e != nil || in.WebSearchRequested {
		t.Fatal("choice inherited from earlier turn")
	}
	raw, e = withWebSearch(s, `{"_webSearch":true}`, false)
	if e != nil || strings.Contains(raw, "_webSearch") {
		t.Fatal("client facts escalated permission")
	}
	s.WebSearchEnabled = false
	if _, e = withWebSearch(s, `{}`, true); e == nil {
		t.Fatal("admin restriction bypassed")
	}
	s.WebSearchEnabled = true
	s.AIConfig.WebSearch.APIKey = ""
	if _, e = withWebSearch(s, `{}`, true); e == nil {
		t.Fatal("unconfigured accepted")
	}
}
