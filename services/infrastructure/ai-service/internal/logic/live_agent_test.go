package logic

import (
	"context"
	"encoding/json"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/cloudwego/eino/schema"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLiveContextKeepsUserFactsAndReportAcrossTurns(t *testing.T) {
	messages := []*model.AIMessage{
		{Id: 1, Role: "assistant", Status: "completed", Content: "报告正文", InputJSON: `{"birthTime":"fake"}`},
		{Id: 2, Role: "user", Status: "completed", Content: "请结合今年再分析", InputJSON: `{"birthDate":"1991-03-04"}`},
		{Id: 3, Role: "assistant", Status: "completed", Content: "请补充出生时间", InputJSON: `{"birthTime":"guessed"}`},
		{Id: 4, Role: "user", Status: "completed", Content: "我已确认补充资料，请继续刚才的问题", InputJSON: `{"birthTime":"12:30"}`},
		{Id: 5, Role: "assistant", Status: "pending"},
		{Id: 6, Role: "user", Status: "completed", Content: "未来消息不得影响重试", InputJSON: `{"birthTime":"00:00"}`},
	}
	in, e := liveContext(messages, 5, 2, &Report{Title: "报告", Content: "已购内容", InputsJSON: `{"birthDate":"1990-01-02","gender":"male"}`})
	if e != nil || in.Facts["birthDate"] != "1991-03-04" || in.Facts["birthTime"] != "12:30" || in.Facts["gender"] != "male" || in.Question != "请结合今年再分析" || len(in.Messages) != 3 {
		t.Fatalf("lost/mixed context: %+v %v", in, e)
	}
	if strings.Contains(in.Messages[0].Content, "报告正文") {
		t.Fatal("report was not loaded on demand")
	}
	// Rebuilding from persisted messages has identical facts without an in-memory runner.
	rebuilt, e := liveContext(messages, 5, 2, &Report{Title: "报告", Content: "已购内容", InputsJSON: `{"gender":"male"}`})
	if e != nil || rebuilt.Facts["birthTime"] != in.Facts["birthTime"] {
		t.Fatal("restart lost confirmed facts")
	}
}
func TestLiveReportQueryUsesOwnerAndSession(t *testing.T) {
	db, m, _ := sqlmock.New()
	defer db.Close()
	m.ExpectQuery("SELECT id FROM ai_report WHERE chat_session_id=\\? AND user_id=\\? LIMIT 1").WithArgs(int64(12), "owner").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	r, e := liveReport(context.Background(), &svc.ServiceContext{DB: sqlx.NewSqlConnFromDB(db)}, &model.AISession{Id: 12, UserId: "owner"})
	if e != nil || r != nil {
		t.Fatalf("unexpected report: %v %v", r, e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}

func TestImageMessageReachesOpenAICompatibleTransport(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content []struct {
					Type     string `json:"type"`
					Text     string `json:"text"`
					ImageURL *struct {
						URL string `json:"url"`
					} `json:"image_url"`
				} `json:"content"`
			} `json:"messages"`
		}
		if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
			t.Errorf("invalid multimodal request: %v", e)
		} else if len(body.Messages) != 1 || len(body.Messages[0].Content) != 2 || body.Messages[0].Content[0].Text != "合成图像" || body.Messages[0].Content[1].ImageURL == nil {
			t.Error("text or image lost")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"fixture","object":"chat.completion","model":"fixture","choices":[{"index":0,"message":{"role":"assistant","content":"已读取图片"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer server.Close()
	source := provider.NewOpenAICompatible(server.URL, "fixture", "fixture", "")
	source.SetHTTPClient(server.Client())
	source.UseStandardParameters()
	chat, e := provider.NewEinoModel(context.Background(), source, provider.Request{Model: "fixture", MaxTokens: 128})
	if e != nil {
		t.Fatal(e)
	}
	answer, e := chat.Generate(context.Background(), []*schema.Message{imageUserMessage("合成图像", []string{"data:image/png;base64,aGVsbG8="})})
	if e != nil || calls != 1 || answer == nil || answer.Content != "已读取图片" {
		t.Fatalf("multimodal transport failed calls=%d err=%v", calls, e)
	}
}
