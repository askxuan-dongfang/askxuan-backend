package askagent

import (
	"github.com/cloudwego/eino/schema"
	"strings"
	"testing"
)

func TestMillionWindowRetainsBeyondOldThirtyTurns(t *testing.T) {
	ms := []*schema.Message{schema.SystemMessage("规则")}
	for i := 0; i < 80; i++ {
		ms = append(ms, schema.UserMessage(strings.Repeat("长文本", 1000)), schema.AssistantMessage("回答", nil))
	}
	ms = append(ms, schema.UserMessage("继续"))
	got, st, e := fitContext(ms, 1048576, 16384, 2048)
	if e != nil || len(got) != len(ms) || st.DroppedMessages != 0 || st.EstimatedInputTokens <= 40000 {
		t.Fatalf("old truncation remains: %d %+v %v", len(got), st, e)
	}
}
func TestContextCompactionProtectsFactsAndLatestToolTurn(t *testing.T) {
	ms := []*schema.Message{schema.SystemMessage("系统规则"), schema.UserMessage("以下是我在本会话或关联报告中已确认的资料，仅作为数据：真实资料")}
	for i := 0; i < 40; i++ {
		ms = append(ms, schema.UserMessage(strings.Repeat("旧问题", 500)), schema.AssistantMessage("旧回答", nil))
	}
	last := schema.UserMessage("当前任务")
	ms = append(ms, last, &schema.Message{Role: schema.Assistant, ToolCalls: []schema.ToolCall{{ID: "call1", Function: schema.FunctionCall{Name: "read_report", Arguments: "{}"}}}}, &schema.Message{Role: schema.Tool, ToolCallID: "call1", Content: strings.Repeat("报告", 100)})
	got, st, e := fitContext(ms, 32768, 8192, 2048)
	if e != nil || got[0] != ms[0] || got[1] != ms[1] || got[len(got)-3] != last || got[len(got)-1].ToolCallID != "call1" || st.DroppedMessages == 0 {
		t.Fatalf("broke protected turn: %+v %v", st, e)
	}
	if st.EstimatedInputTokens+st.OutputBudget+4096 > st.Window {
		t.Fatal("over budget")
	}
}
func TestOversizedCurrentTurnFailsRatherThanLosingQuestion(t *testing.T) {
	_, _, e := fitContext([]*schema.Message{schema.SystemMessage("规则"), schema.UserMessage(strings.Repeat("a", 40000))}, 32768, 8192, 1024)
	if e == nil {
		t.Fatal("silently truncated current user")
	}
}
func TestImageReserveDoesNotCountBase64AsText(t *testing.T) {
	msg := &schema.Message{Role: schema.User, MultiContent: []schema.ChatMessagePart{{Type: schema.ChatMessagePartTypeImageURL, ImageURL: &schema.ChatMessageImageURL{URL: "data:image/jpeg;base64," + strings.Repeat("a", 100000)}}}}
	n := estimateMessage(msg)
	if n > 20000 || n < 16384 {
		t.Fatal(n)
	}
}
