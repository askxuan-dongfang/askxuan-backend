package logic

import (
	"context"
	"encoding/json"
	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/ai-service/internal/types"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"strings"
	"testing"
	"time"
)

type reportFixtureProvider struct{ provider.Mock }

func (reportFixtureProvider) Complete(context.Context, provider.Request) (*provider.Response, error) {
	content := "## 资料与分析边界\n" + strings.Repeat("这是隔离数据库验收内容，不代表真实模型质量。", 12) + "\n## 个人优势\n梳理已有经验，结合现实情况。\n## 行动建议\n记录一周行动并复盘。"
	raw, _ := json.Marshal(reportBody{Summary: strings.Repeat("基于提供资料梳理问题，明确文化参考边界。", 3), Content: content})
	return &provider.Response{Content: string(raw), Model: "integration-fixture", FinishReason: "stop", PromptTokens: 100, CompletionTokens: 200}, nil
}
func reportTestContext(t *testing.T) *svc.ServiceContext {
	t.Helper()
	dsn := os.Getenv("AI_REPORT_TEST_DSN")
	if dsn == "" {
		t.Skip("isolated AI_REPORT_TEST_DSN not set")
	}
	db := sqlx.NewMysql(dsn)
	return &svc.ServiceContext{DB: db, ConversationModel: model.NewConversationModel(db), SkillModel: model.NewSkillModel(db), UsageModel: model.NewUsageModel(db), Guard: agent.NewGuard(2000, nil), Provider: reportFixtureProvider{}}
}
func TestMySQLReportGenerate(t *testing.T) {
	s := reportTestContext(t)
	s.AIConfig.MinuteRequestLimit = 30
	s.AIConfig.DailyRequestLimit = 100
	ctx := context.Background()
	_, e := s.DB.ExecCtx(ctx, `UPDATE ai_skill SET tool_config='{"enabled":false}' WHERE code='bazi'`)
	if e != nil {
		t.Fatal(e)
	}
	req := ReportRequest{SkillCode: "bazi", Question: "梳理职业方向", Inputs: map[string]interface{}{"birthDate": "1995-06-15", "calendarType": "solar", "gender": "male", "birthplace": "上海"}, RequestKey: "mysql-report-case"}
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
	if r.Title != "八字命理" || r.Status != "ready" || r.Content != "" || r.Unlocked {
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
