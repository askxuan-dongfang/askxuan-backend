package logic

import (
	"encoding/json"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/common"
)

// Choice is persisted with this user turn so retries never inherit a later choice.
func withWebSearch(s *svc.ServiceContext, raw string, enabled bool) (string, error) {
	if enabled && (!s.AIConfig.HarnessEnabled || !s.WebSearchEnabled || !s.AIConfig.WebSearch.Ready()) {
		return "", common.NewBizError(40001, "当前智能体暂未开放联网搜索，请关闭联网后发送")
	}
	facts := map[string]any{}
	if json.Unmarshal([]byte(raw), &facts) != nil || facts == nil {
		facts = map[string]any{}
	}
	delete(facts, "_webSearch")
	if enabled {
		facts["_webSearch"] = true
	}
	b, e := json.Marshal(facts)
	return string(b), e
}
