package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/ai-service/internal/types"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func reportTestContext(t *testing.T) *svc.ServiceContext {
	t.Helper()
	dsn := os.Getenv("AI_REPORT_TEST_DSN")
	if dsn == "" {
		t.Skip("isolated AI_REPORT_TEST_DSN not set")
	}
	db := sqlx.NewMysql(dsn)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content := "## 梦境线索\n" + strings.Repeat("这是隔离数据库验收内容，不代表真实模型质量。", 20) + "\n## 意象整理\n| 意象 | 解释 |\n| --- | --- |\n| 道路 | 个人联想 |\n## 行动建议\n记录与复盘。"
		raw, _ := json.Marshal(reportBody{Summary: strings.Repeat("依据梦境描述整理线索。", 6), Content: content})
		event, _ := json.Marshal(map[string]any{"id": "fixture", "object": "chat.completion.chunk", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"role": "assistant", "content": string(raw)}, "finish_reason": "stop"}}})
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", event)
	}))
	t.Cleanup(server.Close)
	p := provider.NewOpenAICompatible(server.URL, "fixture", "fixture", "")
	p.SetHTTPClient(server.Client())
	p.UseStandardParameters()
	return &svc.ServiceContext{DB: db, ConversationModel: model.NewConversationModel(db), SkillModel: model.NewSkillModel(db), UsageModel: model.NewUsageModel(db), Guard: agent.NewGuard(2000, nil), Provider: p}
}
func TestMySQLReportGenerate(t *testing.T) {
	s := reportTestContext(t)
	s.AIConfig.MinuteRequestLimit = 30
	s.AIConfig.DailyRequestLimit = 100
	ctx := context.Background()
	req := ReportRequest{SkillCode: "dream", Question: "整理梦境中的道路", Inputs: map[string]interface{}{"dream": "在一条长路上散步"}, RequestKey: "mysql-report-case"}
	r, e := ReportCreate(ctx, s, "9501", req)
	if e != nil {
		t.Fatal(e)
	}
	again, e := ReportCreate(ctx, s, "9501", req)
	if e != nil || again.ID != r.ID {
		t.Fatalf("idempotency %v", e)
	}
	for i := 0; i < 50; i++ {
		r, e = ReportGet(ctx, s, "9501", r.ID)
		if e != nil {
			t.Fatal(e)
		}
		if r.Status != "generating" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if r.Title != "周公解梦" || r.Status != "ready" || r.Content != "" || r.Unlocked {
		t.Fatalf("preview boundary %+v", r)
	}
	if _, e = ReportGet(ctx, s, "9502", r.ID); e == nil {
		t.Fatal("other user accessed report")
	}
	if _, e = ReportConversation(ctx, s, "9501", r.ID); e == nil {
		t.Fatal("unpaid followup leaked report")
	}
	rows, e := ReportList(ctx, s, "9501", 1)
	if e != nil || len(rows) != 1 || rows[0].Content != "" {
		t.Fatalf("list %v %v", rows, e)
	}
}
func TestMySQLReportReadPaid(t *testing.T) {
	s := reportTestContext(t)
	ctx := context.Background()
	var id int64
	if e := s.DB.QueryRowCtx(ctx, &id, `SELECT id FROM ai_report WHERE user_id='9501' AND request_key='mysql-report-case'`); e != nil {
		t.Fatal(e)
	}
	if _, e := s.DB.ExecCtx(ctx, `UPDATE ai_report SET points_paid=1 WHERE id=? AND user_id='9501'`, id); e != nil {
		t.Fatal(e)
	}
	r, e := ReportGet(ctx, s, "9501", id)
	if e != nil || !r.Unlocked || r.Content == "" {
		t.Fatalf("paid %+v %v", r, e)
	}
	first, e := ReportConversation(ctx, s, "9501", id)
	if e != nil {
		t.Fatal(e)
	}
	next, e := ReportConversation(ctx, s, "9501", id)
	if e != nil || first["sessionId"] != next["sessionId"] {
		t.Fatalf("followup idempotency %v", e)
	}
	var owner string
	if e = s.DB.QueryRowCtx(ctx, &owner, `SELECT user_id FROM ai_session WHERE id=?`, first["sessionId"]); e != nil || owner != "9501" {
		t.Fatal("conversation owner")
	}
	sid := first["sessionId"]
	deleter := NewSessionDeleteLogic(ctx, s)
	if _, e = deleter.Delete(&types.SessionDeleteReq{Id: sid, UserId: "9502"}); e == nil {
		t.Fatal("foreign deletion succeeded")
	}
	for i := 0; i < 2; i++ {
		if _, e = deleter.Delete(&types.SessionDeleteReq{Id: sid, UserId: "9501"}); e != nil {
			t.Fatalf("idempotent delete: %v", e)
		}
	}
	for _, status := range []string{"", "active", "closed"} {
		rows, err := NewSessionListLogic(ctx, s).List(&types.SessionListReq{UserId: "9501", Status: status})
		if err != nil || rows.Total != 0 {
			t.Fatalf("deleted session listed: %+v %v", rows, err)
		}
	}
	if _, e = NewSessionDetailLogic(ctx, s).Detail(&types.SessionDetailReq{Id: sid, UserId: "9501"}); e == nil {
		t.Fatal("deleted detail accessible")
	}
	if _, e = NewMessageListLogic(ctx, s).List(&types.MessageListReq{Id: sid, UserId: "9501"}); e == nil {
		t.Fatal("deleted messages accessible")
	}
	if _, e = NewMessageSendLogic(ctx, s).Send(&types.MessageSendReq{Id: sid, UserId: "9501", Content: "不得发起生成"}); e == nil {
		t.Fatal("deleted session accepted send")
	}
	messages, _, e := s.ConversationModel.ListMessages(ctx, sid, 1, 100)
	if e != nil || len(messages) == 0 {
		t.Fatal("audit messages lost")
	}
	mid := messages[0].Id
	if _, e = s.ConversationModel.FindMessageForUser(ctx, sid, mid, "9501"); e == nil {
		t.Fatal("deleted stream/trace accessible")
	}
	if _, e = NewMessageRetryLogic(ctx, s).Retry(&types.MessageRetryReq{Id: sid, MessageId: mid, UserId: "9501"}); e == nil {
		t.Fatal("deleted retry accepted")
	}
	if _, e = NewMessageTraceLogic(ctx, s).Trace(&types.MessageTraceReq{Id: sid, MessageId: mid, UserId: "9501"}); e == nil {
		t.Fatal("deleted trace accessible")
	}
	fresh, e := ReportConversation(ctx, s, "9501", id)
	if e != nil || fresh["sessionId"] == sid {
		t.Fatalf("followup resurrected: %v", e)
	}
	again, e := ReportConversation(ctx, s, "9501", id)
	if e != nil || again["sessionId"] != fresh["sessionId"] {
		t.Fatal("replacement followup not idempotent")
	}
	r, e = ReportGet(ctx, s, "9501", id)
	if e != nil || !r.Unlocked || r.Content == "" {
		t.Fatal("purchased report lost")
	}
}
