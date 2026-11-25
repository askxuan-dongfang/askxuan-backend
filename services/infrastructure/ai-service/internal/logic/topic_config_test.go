package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/agentops"
	"github.com/askxuan/ai-service/internal/knowledge"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestReportCreateUsesPublishedFormAndStoresChapters(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	chapters := []string{"观察记录", "归纳", "下一步"}
	s := &svc.ServiceContext{
		DB: sqlx.NewSqlConnFromDB(db), Guard: agent.NewGuard(20000, nil), UsageModel: &retryUsageFixture{}, AgentVersion: 7,
		SkillModel: reportSkills{&model.AISkill{Code: "dream", Status: "enabled", InputSchema: reportToolSchema("dream"), ToolConfig: `{"enabled":true,"server":"builtin","tool":"dream"}`}},
		PublishedAgent: &agentops.Config{Topics: []agentops.TopicPolicy{
			{Code: "dream", ToolCodes: []string{"dream"}, Chapters: chapters, AdditionalFields: []agentops.TopicField{{Key: "topic_context", Label: "近期关注", Required: true}}},
		}},
	}
	stopped := errors.New("fixture stops before async execution")
	req := ReportRequest{SkillCode: "dream", RequestKey: "fixture-key", Question: "整理梦境", Inputs: map[string]any{"dream": "梦见一条道路"}}
	for _, valid := range []bool{false, true} {
		mock.ExpectQuery("SELECT id FROM ai_report WHERE user_id").WithArgs("fixture-user", req.RequestKey).WillReturnRows(sqlmock.NewRows([]string{"id"}))
		mock.ExpectQuery("SELECT p.code,p.title,p.subtitle").WillReturnRows(sqlmock.NewRows([]string{"code", "title", "subtitle", "price_cents", "points_price", "chapters_json", "version", "tool_config"}).
			AddRow("dream", "梦境", "记录", 990, 0, `["原始章节"]`, "3.0", "{}"))
		want := error(common.ErrParamInvalid)
		if valid {
			req.Inputs["topic_context"] = "Work and sleep"
			inputs, _ := json.Marshal(req.Inputs)
			raw, _ := json.Marshal(chapters)
			mock.ExpectExec("INSERT INTO ai_report").WithArgs(sqlmock.AnyArg(), "fixture-user", req.RequestKey, "dream", "梦境", "3.0@a7", req.Question, string(inputs), string(raw), int64(990), int64(0)).
				WillReturnResult(sqlmock.NewErrorResult(stopped))
			want = stopped
		}
		if _, err := ReportCreate(context.Background(), s, "fixture-user", req); !errors.Is(err, want) {
			t.Fatalf("valid=%t: got %v, want %v", valid, err, want)
		}
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

type topicSearchFixture struct{ ids []string }

func (s *topicSearchFixture) Search(_ context.Context, _ string, ids []string) (knowledge.Result, error) {
	s.ids = ids
	return knowledge.Result{Hits: []knowledge.Hit{}}, nil
}

func TestReportTopicScopesToolsKnowledgeSkillsAndRepairsChapterOrder(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, stage := range []string{"checking", "calculating", "writing"} {
		mock.ExpectExec("UPDATE ai_report SET generation_stage").WithArgs(stage, int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	}
	search := &topicSearchFixture{}
	p := provider.NewOpenAICompatible("http://scoped-report.invalid", "fixture", "fixture", "")
	body := reportBody{Summary: "这是根据已确认记录撰写的本地合成报告，用于验证章节顺序和配置范围。", Content: "## 记录线索\n" + strings.Repeat("这段合成文字用于验证专题报告工具和已确认资料。", 18) + "\n## 选项比较\n| 选项 | 核对 |\n| --- | --- |\n| 记录 | 时间 |\n## 后续行动\n记录下一次观察。"}
	writes, reviews := 0, 0
	p.SetHTTPClient(&http.Client{Transport: defaultContextTransport(func(r *http.Request) (*http.Response, error) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			return nil, err
		}
		w := httptest.NewRecorder()
		messages := request["messages"].([]any)
		if isReportCalibrationFixtureRequest(request) {
			reviews++
			var input reportCalibrationInput
			text := fmt.Sprint(messages[1].(map[string]any)["content"])
			if json.Unmarshal([]byte(text), &input) != nil || !strings.Contains(string(input.Calculations), `"tool":"naming"`) {
				t.Error("calibration did not receive the optional confirmed calculation")
			}
			writeReportCalibrationFixture(w, `{"issues":[]}`)
		} else {
			writes++
			system := fmt.Sprint(messages[0].(map[string]any)["content"])
			if !strings.Contains(system, "专题专用技能规则") || strings.Contains(system, "未选技能规则") || !strings.Contains(system, "近期关注") {
				t.Error("writer instruction did not use the published skill/field selection")
			}
			delta := map[string]any{"role": "assistant"}
			finish := "stop"
			if writes == 1 {
				tools, _ := json.Marshal(request["tools"])
				for _, name := range []string{"calculate_dream", "calculate_naming", "search_knowledge"} {
					if !strings.Contains(string(tools), name) {
						t.Errorf("missing allowed tool %s", name)
					}
				}
				if strings.Contains(string(tools), "calculate_qimen") {
					t.Error("unselected MCP tool reached the writer")
				}
				delta["tool_calls"] = []any{
					map[string]any{"index": 0, "id": "lookup-1", "type": "function", "function": map[string]any{"name": "search_knowledge", "arguments": `{"query":"已审核资料"}`}},
					map[string]any{"index": 1, "id": "calc-1", "type": "function", "function": map[string]any{"name": "calculate_naming", "arguments": `{}`}},
				}
				finish = "tool_calls"
			} else {
				draft := body
				if writes == 2 {
					draft.Content = strings.Replace(draft.Content, "## 选项比较", "## 其他", 1)
				} else if !strings.Contains(fmt.Sprint(messages[len(messages)-1]), "正文须按顺序") {
					t.Error("repair did not receive the chapter validation error")
				}
				raw, _ := json.Marshal(draft)
				delta["content"] = string(raw)
			}
			event, _ := json.Marshal(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}})
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", event)
		}
		response := w.Result()
		response.Request = r
		return response, nil
	})})
	skills := reportSkills{}
	for _, code := range []string{"dream", "naming", "qimen"} {
		skills = append(skills, &model.AISkill{Code: code, Name: code, Status: model.SkillStatusEnabled, InputSchema: reportToolSchema(code), ToolConfig: fmt.Sprintf(`{"enabled":true,"server":"builtin","tool":%q}`, code)})
	}
	s := &svc.ServiceContext{
		DB: sqlx.NewSqlConnFromDB(db), SkillModel: skills, Guard: agent.NewGuard(20000, nil), Provider: p, Knowledge: &knowledge.Store{Remote: search},
		KnowledgeEnabled: true, KnowledgeBases: []string{"topic-library", "unrelated-library"},
		PublishedAgent: &agentops.Config{
			Skills: []agentops.SkillPolicy{{Code: "selected", Enabled: true, Prompt: "专题专用技能规则"}, {Code: "unselected", Enabled: true, Prompt: "未选技能规则"}},
			Topics: []agentops.TopicPolicy{{Code: "dream", ToolCodes: []string{"dream", "naming"}, KnowledgeBaseIDs: []string{"topic-library"},
				SkillCodes: []string{"selected"}, AdditionalFields: []agentops.TopicField{{Key: "topic_context", Label: "近期关注"}}}},
		},
	}
	s.AIConfig.ContextWindow = 1048576
	r := &Report{ID: 1, SkillCode: "dream", UserID: "synthetic", Question: "整理专题参考",
		InputsJSON:   `{"dream":"梦见一条道路","topic_context":"Work and sleep","mode":"review","surname":"李","name":"李明","eventTime":"2026-11-25T10:00"}`,
		ChaptersJSON: `["记录线索","选项比较","后续行动"]`}
	got, _, _, err := executeReport(context.Background(), s, r)
	if err != nil || got != body || writes != 3 || reviews != 2 {
		t.Fatalf("topic report did not follow the configured format: err=%v writes=%d reviews=%d", err, writes, reviews)
	}
	if !reflect.DeepEqual(search.ids, []string{"topic-library"}) || len(s.KnowledgeBases) != 2 {
		t.Fatal("report escaped topic search scope or changed the agent's global scope")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
