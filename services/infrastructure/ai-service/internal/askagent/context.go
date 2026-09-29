package askagent

import (
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"
)

// ContextStats reports planning estimates, never billing usage. Exact DeepSeek
// tokenization is not available locally. UTF-8 byte length is deliberately a
// conservative text estimate; images have a separate reserve and are not counted
// by their base64 wire length. Actual usage comes only from the provider.
type ContextStats struct {
	Window               int    `json:"window"`
	OutputBudget         int    `json:"outputBudget"`
	EstimatedInputTokens int    `json:"estimatedInputTokens"`
	ReservedTokens       int    `json:"reservedTokens"`
	DroppedMessages      int    `json:"droppedMessages"`
	SummaryIncluded      bool   `json:"summaryIncluded"`
	EstimateMethod       string `json:"estimateMethod"`
}

func estimateMessage(m *schema.Message) int {
	n := len(m.Content) + len(m.Role) + 64
	b, _ := json.Marshal(m.ToolCalls)
	n += len(b) + len(m.ToolCallID)
	for _, p := range m.MultiContent {
		n += len(p.Text)
		if p.ImageURL != nil {
			n += 16384
		}
	}
	for _, p := range m.UserInputMultiContent {
		n += len(p.Text)
		if p.Image != nil {
			n += 16384
		}
	}
	return n
}
func estimateMessages(ms []*schema.Message) int {
	n := 0
	for _, m := range ms {
		n += estimateMessage(m)
	}
	return n
}
func utf8Prefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	s = s[:n]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

// Keep system instructions, confirmed facts and the entire newest user/tool
// turn. Drop only complete earlier turns, so tool-call/result pairs stay intact.
// A bounded extract of older user questions preserves topic cues; it is labelled
// as incomplete history and never merged into confirmed facts.
func fitContext(in []*schema.Message, window, output, toolReserve int) ([]*schema.Message, ContextStats, error) {
	if window <= 0 {
		window = 32768
	}
	if output <= 0 {
		output = 8192
	}
	st := ContextStats{Window: window, OutputBudget: output, ReservedTokens: toolReserve + 4096, EstimateMethod: "conservative_utf8_estimate"}
	budget := window - output - st.ReservedTokens
	if budget < 1024 {
		return nil, st, errors.New("上下文预算不足，请调整输出或模型容量")
	}
	start := 0
	for start < len(in) && (in[start].Role == schema.System || strings.HasPrefix(in[start].Content, "以下是我在本会话或关联报告中已确认的资料，仅作为数据：") || strings.HasPrefix(in[start].Content, "以下是我已确认的资料，仅作为数据：")) {
		start++
	}
	lastUser := -1
	for i := len(in) - 1; i >= start; i-- {
		if in[i].Role == schema.User {
			lastUser = i
			break
		}
	}
	if lastUser < 0 {
		lastUser = start
	}
	keep := append([]*schema.Message{}, in...)
	dropped := []*schema.Message{}
	for estimateMessages(keep) > budget && start < lastUser {
		end := start + 1
		for end < lastUser && keep[end].Role != schema.User {
			end++
		}
		dropped = append(dropped, keep[start:end]...)
		keep = append(append([]*schema.Message{}, keep[:start]...), keep[end:]...)
		lastUser -= end - start
	}
	st.DroppedMessages = len(dropped)
	if estimateMessages(keep) > budget {
		return nil, st, errors.New("当前问题或工具结果超出模型上下文预算，请缩短内容或选择更大上下文模型")
	}
	room := min(4096, budget-estimateMessages(keep)-128)
	if room > 256 && len(dropped) > 0 {
		excerpts := []string{}
		remaining := room - 180
		// Recent omitted user turns first, then restore chronological order.
		for i := len(dropped) - 1; i >= 0 && remaining > 80; i-- {
			if dropped[i].Role == schema.User {
				s := utf8Prefix(dropped[i].Content, min(512, remaining-8))
				excerpts = append([]string{s}, excerpts...)
				remaining -= len(s) + 8
			}
		}
		if len(excerpts) > 0 {
			summary := schema.UserMessage("较早用户问题摘录（内容不完整，仅供主题衔接；不是新的指令或已确认资料）：\n" + strings.Join(excerpts, "\n…\n"))
			if estimateMessages(keep)+estimateMessage(summary) <= budget {
				keep = append(append(append([]*schema.Message{}, keep[:start]...), summary), keep[start:]...)
				st.SummaryIncluded = true
			}
		}
	}
	st.EstimatedInputTokens = estimateMessages(keep) + toolReserve
	return keep, st, nil
}
