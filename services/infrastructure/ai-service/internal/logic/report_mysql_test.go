package logic

import (
	"context"
	"database/sql"
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
	"sync/atomic"
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
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		if isReportCalibrationFixtureRequest(req) {
			writeReportCalibrationFixture(w, `{"issues":[]}`)
			return
		}
		content := "## 梦境线索\n" + strings.Repeat("这是隔离数据库验收内容，不代表真实模型质量。", 20) + "\n## 意象整理\n| 意象 | 解释 |\n| --- | --- |\n| 道路 | 个人联想 |\n## 行动建议\n记录与复盘。"
		raw, _ := json.Marshal(reportBody{Summary: strings.Repeat("依据梦境描述整理线索。", 6), Content: content})
		delta := map[string]any{"role": "assistant", "content": string(raw)}
		reason := "stop"
		if calls.Add(1) == 1 {
			reason = "tool_calls"
			delta = map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": "read_1", "type": "function", "function": map[string]any{"name": "read_report", "arguments": "{}"}}}}
		}
		event, _ := json.Marshal(map[string]any{"id": "fixture", "object": "chat.completion.chunk", "model": "fixture", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": reason}}})
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
	var generated struct {
		DocumentJSON     string `db:"document_json"`
		PromptTokens     int    `db:"prompt_tokens"`
		CompletionTokens int    `db:"completion_tokens"`
	}
	if e = s.DB.QueryRowCtx(ctx, &generated, `SELECT document_json,prompt_tokens,completion_tokens FROM ai_report WHERE id=?`, r.ID); e != nil {
		t.Fatal(e)
	}
	var document struct {
		ModelCalls int `json:"modelCalls"`
	}
	if e = json.Unmarshal([]byte(generated.DocumentJSON), &document); e != nil || document.ModelCalls != 3 || generated.PromptTokens != 11 || generated.CompletionTokens != 7 {
		t.Fatalf("persisted calibration usage: %+v document=%+v err=%v", generated, document, e)
	}
	if _, e = ReportGet(ctx, s, "9502", r.ID); e == nil {
		t.Fatal("other user accessed report")
	}
	if _, e = ReportConversation(ctx, s, "9501", r.ID); e == nil {
		t.Fatal("unpaid followup leaked report")
	}
	if _, e = s.DB.ExecCtx(ctx, `INSERT INTO ai_report(report_no,user_id,request_key,skill_code,title,version,question,inputs_json,chapters_json,price_cents,points_price,status,summary,content) VALUES('ARfixture-demo','9503','mysql-report-demo-case','dream','demo wallet fixture','1','fixture','{}','[]',990,0,'ready','fixture summary','fixture content')`); e != nil {
		t.Fatal(e)
	}
	rows, e := ReportList(ctx, s, "9501", 1)
	if e != nil || len(rows) != 1 || rows[0].Content != "" {
		t.Fatalf("list %v %v", rows, e)
	}
}
func TestMySQLReportReadPaid(t *testing.T) {
	s := reportTestContext(t)
	ctx := context.Background()
	var demoID int64
	if e := s.DB.QueryRowCtx(ctx, &demoID, `SELECT id FROM ai_report WHERE report_no='ARfixture-demo'`); e != nil {
		t.Fatal(e)
	}
	demo, e := ReportGet(ctx, s, "9503", demoID)
	if e != nil || !demo.Unlocked || demo.Content == "" {
		t.Fatal("funded demo denied", e)
	}

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

func TestMySQLReportReadRefunded(t *testing.T) {
	s := reportTestContext(t)
	ctx := context.Background()
	var id int64
	if e := s.DB.QueryRowCtx(ctx, &id, `SELECT id FROM ai_report WHERE user_id='9501' AND request_key='mysql-report-case'`); e != nil {
		t.Fatal(e)
	}
	r, e := ReportGet(ctx, s, "9501", id)
	if e != nil || r.Unlocked || r.Content != "" || r.Document != nil {
		t.Fatal("refund retained content", e)
	}
	if _, e = ReportConversation(ctx, s, "9501", id); e == nil {
		t.Fatal("refund retained followup access")
	}
	if e = s.DB.QueryRowCtx(ctx, &id, `SELECT id FROM ai_report WHERE report_no='ARfixture-demo'`); e != nil {
		t.Fatal(e)
	}
	r, e = ReportGet(ctx, s, "9503", id)
	if e != nil || r.Unlocked || r.Content != "" || r.Document != nil {
		t.Fatal("demo refund retained access", e)
	}
}

func TestMySQLReportEntitlementMatrix(t *testing.T) {
	s := reportTestContext(t)
	ctx := context.Background()
	dsn := os.Getenv("ROOT_TEST_DSN")
	if dsn == "" {
		t.Skip("isolated root fixture DSN missing")
	}
	db, e := sql.Open("mysql", dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	exec := func(q string, args ...any) sql.Result {
		t.Helper()
		r, e := db.ExecContext(ctx, q, args...)
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	cases := []struct {
		name, channel, refund    string
		price, paid              int64
		growth, ledger, unlocked bool
	}{
		{"discount", "balance", "", 990, 800, true, false, true},
		{"unproven-discount", "balance", "", 990, 800, false, false, false},
		{"pending", "balance", "pending", 990, 990, false, false, false},
		{"processing", "balance", "processing", 990, 990, false, false, false},
		{"success", "balance", "success", 990, 990, false, false, false},
		{"failed-refund", "balance", "failed", 990, 990, false, false, true},
		{"free-mock", "mock", "", 990, 990, false, false, false},
		{"unfunded-demo", "demo_balance", "", 990, 990, false, false, false},
		{"funded-demo", "demo_balance", "", 990, 990, false, true, true},
		{"historical-wechat", "wechat", "", 990, 990, false, false, true},
	}
	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			no := fmt.Sprintf("ARmatrix-%d", i)
			payno := fmt.Sprintf("Pmatrix-%d", i)
			id, _ := exec(`INSERT INTO askxuan_ai.ai_report(report_no,user_id,request_key,skill_code,title,version,question,inputs_json,chapters_json,price_cents,points_price,status,summary,content) VALUES(?,'matrix-owner',?,'dream','matrix','1','fixture','{}','[]',?,0,'ready','summary','private fixture')`, no, no, c.price).LastInsertId()
			pid, _ := exec(`INSERT INTO askxuan_payment.payment(payment_no,idempotency_key,user_id,order_type,order_no,amount,channel,status,trade_no) VALUES(?,?,'matrix-owner','ai_report',?,?/100,?,'success','fixture')`, payno, "ai_report:"+no, no, c.paid, c.channel).LastInsertId()
			if c.growth {
				exec(`INSERT INTO askxuan_payment.growth_transaction(payment_id,user_id,order_type,order_no,gross_cents,paid_cents,growth_base_cents,next_check_at,created_at) VALUES(?,'matrix-owner','ai_report',?,?,?,0,0,0)`, pid, no, c.price, c.paid)
			}
			if c.ledger {
				exec(`INSERT INTO askxuan_payment.demo_wallet_ledger(user_id,event_key,kind,reference_no,available_delta,held_delta,available_after,held_after) VALUES('matrix-owner',?,'payment',?, ?,0,0,0)`, "pay:"+payno, payno, -c.paid)
			}
			if c.refund != "" {
				exec(`INSERT INTO askxuan_payment.refund(refund_no,payment_id,amount,reason,status) VALUES(?,?,?/100,'fixture',?)`, "R"+payno, pid, c.paid, c.refund)
			}
			got, e := ReportGet(ctx, s, "matrix-owner", id)
			if e != nil || got.Unlocked != c.unlocked || ((got.Content != "") != c.unlocked) {
				t.Fatalf("unlocked=%v want=%v err=%v", got, c.unlocked, e)
			}
			if !c.unlocked {
				if _, e = ReportConversation(ctx, s, "matrix-owner", id); e == nil {
					t.Fatal("locked followup allowed")
				}
			}
		})
	}
	// A legacy points marker alone no longer grants access.
	id, _ := exec(`INSERT INTO askxuan_ai.ai_report(report_no,user_id,request_key,skill_code,title,version,question,inputs_json,chapters_json,price_cents,points_price,status,summary,content,points_paid) VALUES('ARmatrix-points','matrix-owner','matrix-points','dream','matrix','1','fixture','{}','[]',990,10,'ready','summary','private',1)`).LastInsertId()
	got, e := ReportGet(ctx, s, "matrix-owner", id)
	if e != nil || got.Unlocked || got.Content != "" || got.PointsPrice != 0 {
		t.Fatal("points entitlement retained", e)
	}
	// Remove this non-retirement fixture before the approved cleanup count test.
	exec(`DELETE FROM askxuan_ai.ai_report WHERE id=?`, id)
}
