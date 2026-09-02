package agent

import (
	"testing"

	"github.com/askxuan/ai-service/internal/model"
)

func TestRouteSkill(t *testing.T) {
	skills := []*model.AISkill{
		{Code: "general", Status: model.SkillStatusEnabled, RoutingKeywords: `[]`},
		{Code: "bazi", Status: model.SkillStatusEnabled, RoutingKeywords: `["八字","四柱"]`},
		{Code: "tarot", Status: model.SkillStatusEnabled, RoutingKeywords: `["塔罗","抽牌"]`},
	}
	if got := RouteSkill("请用四柱看看事业", nil, skills); got != "bazi" {
		t.Fatalf("expected bazi, got %s", got)
	}
	if got := RouteSkill("今天该注意什么", nil, skills); got != "general" {
		t.Fatalf("expected safe fallback, got %s", got)
	}
	if got := RouteSkill("帮我看看", map[string]interface{}{"spread": "single"}, skills); got != "tarot" {
		t.Fatalf("expected tarot from structured input, got %s", got)
	}
}

func TestRouteSkillTieFallsBackToGeneral(t *testing.T) {
	skills := []*model.AISkill{
		{Code: "general", Status: model.SkillStatusEnabled, RoutingKeywords: `[]`},
		{Code: "one", Status: model.SkillStatusEnabled, RoutingKeywords: `["问事"]`},
		{Code: "two", Status: model.SkillStatusEnabled, RoutingKeywords: `["问事"]`},
	}
	if got := RouteSkill("请帮我问事", nil, skills); got != "general" {
		t.Fatalf("expected tie to use general, got %s", got)
	}
}
