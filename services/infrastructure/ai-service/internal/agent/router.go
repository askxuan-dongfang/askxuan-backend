package agent

import (
	"encoding/json"
	"strings"

	"github.com/askxuan/ai-service/internal/model"
)

// RouteSkill performs deterministic routing over server-reviewed keywords. A tie
// or weak match falls back to general so the model never silently guesses a
// divination method that the user did not ask for.
func RouteSkill(question string, inputs map[string]interface{}, skills []*model.AISkill) string {
	text := strings.ToLower(strings.TrimSpace(question))
	bestCode, bestScore := model.SkillCodeGeneral, 0
	tied := false
	for _, skill := range skills {
		if skill.Status != model.SkillStatusEnabled || skill.Code == model.SkillCodeGeneral {
			continue
		}
		var keywords []string
		if json.Unmarshal([]byte(skill.RoutingKeywords), &keywords) != nil {
			continue
		}
		score := 0
		for _, keyword := range keywords {
			if normalized := strings.ToLower(strings.TrimSpace(keyword)); normalized != "" && strings.Contains(text, normalized) {
				score += len([]rune(normalized))
			}
		}
		if score > bestScore {
			bestCode, bestScore = skill.Code, score
			tied = false
		} else if score > 0 && score == bestScore {
			tied = true
		}
	}
	if tied {
		return model.SkillCodeGeneral
	}
	if bestScore == 0 && len(inputs) > 0 {
		if _, ok := inputs["spread"]; ok {
			return "tarot"
		}
		if _, ok := inputs["eventTime"]; ok {
			return "qimen"
		}
		if _, ok := inputs["birthDate"]; ok {
			return "bazi"
		}
	}
	return bestCode
}
